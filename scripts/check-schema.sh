#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'USAGE'
Usage:
  scripts/check-schema.sh deploy/schema.sql
  scripts/check-schema.sh deploy/schema.sql --mysql [mysql client options] DATABASE

The optional MySQL mode requires an already-created, empty MySQL 8 database.
Connection options are passed directly to the mysql client. Prefer a protected
--defaults-extra-file instead of putting a password on the command line.

Example:
  scripts/check-schema.sh deploy/schema.sql --mysql \
    --defaults-extra-file=/run/secrets/mysql-client.cnf kirby_schema_test
USAGE
  exit 2
}

fail() {
  echo "schema check failed: $*" >&2
  exit 1
}

[[ $# -ge 1 ]] || usage

schema_file=$1
[[ -f "$schema_file" ]] || fail "file does not exist: $schema_file"

script_dir=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_dir=$(CDPATH='' cd -- "$script_dir/.." && pwd)
server_dir="$repo_dir/server"

expected_tables='audit_logs
config_enums
configs
environments
import_records
permissions
project_api_keys
projects
refresh_tokens
role_permissions
roles
snapshots
structures
user_environment_roles
users'

tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/kirby-schema-check.XXXXXX")
trap 'rm -rf -- "$tmp_dir"' EXIT

printf '%s\n' "$expected_tables" | sort >"$tmp_dir/expected-tables"
sed -nE 's/^[[:space:]]*CREATE TABLE `([^`]*)`.*/\1/p' "$schema_file" | sort >"$tmp_dir/actual-tables"

if ! diff -u "$tmp_dir/expected-tables" "$tmp_dir/actual-tables"; then
  fail "CREATE TABLE inventory differs from the fixed schema"
fi

table_count=$(wc -l <"$tmp_dir/actual-tables" | tr -d '[:space:]')
[[ "$table_count" = "15" ]] || fail "expected 15 tables, found $table_count"

if grep -Eq 'CREATE[[:space:]]+TABLE[[:space:]]+IF[[:space:]]+NOT[[:space:]]+EXISTS' "$schema_file"; then
  fail "schema must fail when executed a second time; remove IF NOT EXISTS"
fi

required_patterns=(
  'UNIQUE KEY `ux_environments_key` \(`key`\)'
  'UNIQUE KEY `ux_users_username` \(`username`\)'
  'UNIQUE KEY `ux_roles_key` \(`key`\)'
  'UNIQUE KEY `ux_permissions_key` \(`key`\)'
  'UNIQUE KEY `ux_user_environment_roles_assignment` \(`user_id`, `environment_id`, `role_id`\)'
  'UNIQUE KEY `ux_role_permissions_assignment` \(`role_id`, `permission_id`\)'
  'UNIQUE KEY `ux_projects_environment_key` \(`environment_id`, `key`\)'
  'UNIQUE KEY `ux_configs_project_key` \(`project_id`, `key`\)'
  'UNIQUE KEY `ux_structures_config_key` \(`config_id`, `key`\)'
  'UNIQUE KEY `ux_config_enums_config_key` \(`config_id`, `key`\)'
  'UNIQUE KEY `ux_refresh_tokens_hash` \(`token_hash`\)'
  'UNIQUE KEY `ux_project_api_keys_public_id` \(`public_id`\)'
  'UNIQUE KEY `ux_import_records_idempotency` \(`user_id`, `target_environment_id`, `idempotency_key`\)'
  '`runtime_version` BIGINT NOT NULL DEFAULT 0'
  '`is_system_admin` BOOLEAN NOT NULL DEFAULT FALSE'
  '`token_hash` BINARY\(32\) NOT NULL'
  '`secret_hash` BINARY\(32\) NOT NULL'
  '`secret_suffix` CHAR\(4\).*NOT NULL'
  '`request_hash` BINARY\(32\) NOT NULL'
  "CHECK \(\`status\` IN \(1, 3\)\)"
  "CHECK \(\`status\` IN \('pending', 'succeeded', 'failed'\)\)"
)

for pattern in "${required_patterns[@]}"; do
  grep -Eq "$pattern" "$schema_file" || fail "missing required schema fragment: $pattern"
done

for permission in \
  project:read project:write project:api_key:read project:api_key:manage \
  config:read config:write structure:read structure:write enum:read enum:write \
  snapshot:read snapshot:write snapshot:publish snapshot:export snapshot:import \
  asset:write environment:member:manage system:user:manage system:role:manage \
  system:environment:manage; do
  grep -Fq "'$permission'" "$schema_file" || fail "missing seeded permission: $permission"
done

for role in viewer editor publisher admin; do
  grep -Eq "\([0-9]+, '$role',[[:space:]]" "$schema_file" || fail "missing built-in role: $role"
done

if grep -R -n -E --include='*.go' --exclude='*_test.go' \
  '(Sync2[[:space:]]*\(|CreateTables[[:space:]]*\(|AutoMigrate[[:space:]]*\(|CREATE[[:space:]]+TABLE)' \
  "$server_dir" >"$tmp_dir/ddl-calls"; then
  cat "$tmp_dir/ddl-calls" >&2
  fail "service code contains an automatic DDL path"
fi

if grep -Eq '`(refresh_token|api_key_secret|secret|token)`[[:space:]]' "$schema_file"; then
  fail "plaintext token or API key column found"
fi

echo "schema static check passed: 15 tables, required constraints and seeds present"

if [[ $# -eq 1 ]]; then
  exit 0
fi

[[ ${2:-} = "--mysql" ]] || usage
shift 2
[[ $# -ge 1 ]] || usage
command -v mysql >/dev/null 2>&1 || fail "mysql client is required for --mysql"

mysql_args=("$@")
mysql_version=$(mysql --batch --skip-column-names "${mysql_args[@]}" -e 'SELECT VERSION()') \
  || fail "cannot connect to MySQL"
mysql_major=${mysql_version%%.*}
[[ "$mysql_major" = "8" ]] || fail "MySQL 8 is required, found $mysql_version"

database_name=$(mysql --batch --skip-column-names "${mysql_args[@]}" -e 'SELECT DATABASE()') \
  || fail "cannot determine selected database"
[[ -n "$database_name" && "$database_name" != "NULL" ]] || fail "a database must be selected"

existing_tables=$(mysql --batch --skip-column-names "${mysql_args[@]}" \
  -e 'SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()') \
  || fail "cannot inspect selected database"
[[ "$existing_tables" = "0" ]] || fail "database $database_name is not empty"

mysql --batch --skip-column-names "${mysql_args[@]}" <"$schema_file" \
  || fail "schema execution failed"

created_tables=$(mysql --batch --skip-column-names "${mysql_args[@]}" \
  -e 'SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()')
[[ "$created_tables" = "15" ]] || fail "expected 15 created tables, found $created_tables"

seed_counts=$(mysql --batch --skip-column-names "${mysql_args[@]}" \
  -e "SELECT CONCAT((SELECT COUNT(*) FROM permissions), ':', (SELECT COUNT(*) FROM roles WHERE builtin = TRUE))")
[[ "$seed_counts" = "20:4" ]] || fail "expected 20 permissions and 4 built-in roles, found $seed_counts"

if mysql --batch --skip-column-names "${mysql_args[@]}" <"$schema_file" \
  >"$tmp_dir/second-run.out" 2>"$tmp_dir/second-run.err"; then
  fail "second schema execution unexpectedly succeeded"
fi

echo "schema MySQL check passed: created once and rejected the second execution"
