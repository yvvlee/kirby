-- Kirby schema for PostgreSQL.
--
-- The service runs this embedded schema for an empty database and generates
-- installation secrets after schema creation. This file is also a schema
-- reference. IF NOT EXISTS is omitted so incomplete initialization fails early.


CREATE TABLE "application_secrets" (
  "id" BIGINT NOT NULL,
  "jwt_key_id" VARCHAR(32) NOT NULL,
  "jwt_signing_key" VARCHAR(64) NOT NULL,
  "api_key_pepper" VARCHAR(64) NOT NULL,
  "admin_initialized" BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY ("id"),
  CONSTRAINT "ck_application_secrets_singleton" CHECK ("id" = 1)
);

CREATE TABLE "application_settings" (
  "id" BIGINT NOT NULL,
  "allow_self_registration" BOOLEAN NOT NULL DEFAULT FALSE,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY ("id"),
  "version" BIGINT NOT NULL DEFAULT 0,
  CONSTRAINT "ck_application_settings_singleton" CHECK ("id" = 1)
);

CREATE TABLE "environments" (
  "id" BIGSERIAL NOT NULL,
  "project_id" BIGINT NOT NULL,
  "key" VARCHAR(64) NOT NULL,
  "name" VARCHAR(64) NOT NULL,
  "description" VARCHAR(255) NOT NULL DEFAULT '',
  "enabled" BOOLEAN NOT NULL DEFAULT TRUE,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_environments_project_key" UNIQUE ("project_id", "key")
);

CREATE TABLE "users" (
  "id" BIGSERIAL NOT NULL,
  "username" VARCHAR(128) NOT NULL,
  "email" VARCHAR(320) NOT NULL,
  "nickname" VARCHAR(128) NOT NULL,
  "password_hash" VARCHAR(255) NOT NULL,
  "enabled" BOOLEAN NOT NULL DEFAULT TRUE,
  "is_system_admin" BOOLEAN NOT NULL DEFAULT FALSE,
  "must_change_password" BOOLEAN NOT NULL DEFAULT FALSE,
  "last_login_at" TIMESTAMP(6) NULL,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_users_username" UNIQUE ("username"),
  CONSTRAINT "ux_users_email" UNIQUE ("email")
);





CREATE TABLE "refresh_tokens" (
  "id" BIGSERIAL NOT NULL,
  "user_id" BIGINT NOT NULL,
  "session_id" VARCHAR(64) NOT NULL,
  "token_hash" BYTEA NOT NULL,
  "expires_at" TIMESTAMP(6) NOT NULL,
  "last_used_at" TIMESTAMP(6) NULL,
  "revoked_at" TIMESTAMP(6) NULL,
  "replaced_by_id" BIGINT NULL,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_refresh_tokens_hash" UNIQUE ("token_hash")
);

CREATE TABLE "projects" (
  "id" BIGSERIAL NOT NULL,
  "key" VARCHAR(128) NOT NULL,
  "name" VARCHAR(64) NOT NULL,
  "description" VARCHAR(255) NOT NULL DEFAULT '',
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_projects_key" UNIQUE ("key")
);

CREATE TABLE "project_api_keys" (
  "environment_id" BIGINT NOT NULL,
  "id" BIGSERIAL NOT NULL,
  "project_id" BIGINT NOT NULL,
  "name" VARCHAR(64) NOT NULL,
  "secret_hash" BYTEA NOT NULL,
  "secret_suffix" CHAR(4) NOT NULL,
  "created_by" BIGINT NOT NULL,
  "last_used_at" TIMESTAMP(6) NULL,
  "revoked_at" TIMESTAMP(6) NULL,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY ("id"),
);

CREATE TABLE "configs" (
  "environment_id" BIGINT NOT NULL,
  "id" BIGSERIAL NOT NULL,
  "project_id" BIGINT NOT NULL,
  "key" VARCHAR(128) NOT NULL,
  "description" VARCHAR(255) NOT NULL DEFAULT '',
  "is_array" BOOLEAN NOT NULL DEFAULT FALSE,
  "type_json" JSON NOT NULL,
  "value" TEXT NOT NULL,
  "runtime_version" BIGINT NOT NULL DEFAULT 0,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_configs_environment_key" UNIQUE ("environment_id", "key"),
  CONSTRAINT "ck_configs_runtime_version" CHECK ("runtime_version" >= 0)
);

CREATE TABLE "structures" (
  "id" BIGSERIAL NOT NULL,
  "config_id" BIGINT NOT NULL,
  "key" VARCHAR(128) NOT NULL,
  "name" VARCHAR(64) NOT NULL,
  "description" VARCHAR(255) NOT NULL DEFAULT '',
  "fields_json" JSON NOT NULL,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_structures_config_key" UNIQUE ("config_id", "key")
);

CREATE TABLE "config_enums" (
  "id" BIGSERIAL NOT NULL,
  "config_id" BIGINT NOT NULL,
  "key" VARCHAR(128) NOT NULL,
  "name" VARCHAR(64) NOT NULL,
  "description" VARCHAR(255) NOT NULL DEFAULT '',
  "values_json" JSON NOT NULL,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_config_enums_config_key" UNIQUE ("config_id", "key")
);

CREATE TABLE "snapshots" (
  "id" BIGSERIAL NOT NULL,
  "project_id" BIGINT NOT NULL,
  "config_id" BIGINT NOT NULL,
  "config_key" VARCHAR(128) NOT NULL,
  "description" VARCHAR(255) NOT NULL,
  "content" TEXT NOT NULL,
  "status" SMALLINT NOT NULL DEFAULT 1,
  "tags_json" JSON NOT NULL,
  "is_using" BOOLEAN NOT NULL DEFAULT FALSE,
  "published_at" TIMESTAMP(6) NULL,
  "published_by" BIGINT NULL,
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY ("id"),
  CONSTRAINT "ck_snapshots_status" CHECK ("status" IN (1, 3))
);

CREATE TABLE "import_records" (
  "id" BIGSERIAL NOT NULL,
  "user_id" BIGINT NOT NULL,
  "source_environment_id" BIGINT NOT NULL,
  "target_environment_id" BIGINT NOT NULL,
  "source_snapshot_id" BIGINT NOT NULL,
  "target_project_id" BIGINT NOT NULL,
  "target_snapshot_id" BIGINT NULL,
  "idempotency_key" VARCHAR(128) NOT NULL,
  "request_hash" BYTEA NOT NULL,
  "status" VARCHAR(16) NOT NULL,
  "result_json" JSON NULL,
  "error_message" VARCHAR(1024) NOT NULL DEFAULT '',
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "updated_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "version" BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY ("id"),
  CONSTRAINT "ux_import_records_idempotency" UNIQUE ("user_id", "target_environment_id", "idempotency_key"),
  CONSTRAINT "ck_import_records_status" CHECK ("status" IN ('pending', 'succeeded', 'failed'))
);

CREATE TABLE "audit_logs" (
  "id" BIGSERIAL NOT NULL,
  "actor_user_id" BIGINT NULL,
  "environment_id" BIGINT NULL,
  "action" VARCHAR(128) NOT NULL,
  "resource_type" VARCHAR(64) NOT NULL,
  "resource_id" VARCHAR(128) NOT NULL DEFAULT '',
  "result" VARCHAR(16) NOT NULL,
  "request_id" VARCHAR(128) NOT NULL,
  "details_json" JSON NULL,
  "created_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  "updated_at" TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY ("id"),
  CONSTRAINT "ck_audit_logs_result" CHECK ("result" IN ('succeeded', 'failed'))
);


CREATE TABLE "project_members" (
  "id" BIGSERIAL PRIMARY KEY,
  "project_id" BIGINT NOT NULL,
  "user_id" BIGINT NOT NULL,
  "role" VARCHAR(16) NOT NULL CHECK ("role" IN ('OWNER', 'MAINTAINER', 'MEMBER', 'GUEST')),
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "version" BIGINT NOT NULL DEFAULT 0,
  CONSTRAINT "ux_project_members_project_user" UNIQUE ("project_id", "user_id")
);
CREATE INDEX "ix_project_members_user" ON "project_members" ("user_id");

CREATE TABLE "environment_permission_grants" (
  "id" BIGSERIAL PRIMARY KEY,
  "environment_id" BIGINT NOT NULL,
  "role" VARCHAR(16) NOT NULL CHECK ("role" IN ('OWNER', 'MAINTAINER', 'MEMBER', 'GUEST')),
  "permission" VARCHAR(32) NOT NULL CHECK ("permission" IN ('READ', 'WRITE', 'PUBLISH', 'API_KEY_MANAGE')),
  "created_by" BIGINT NOT NULL DEFAULT 0,
  "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "version" BIGINT NOT NULL DEFAULT 0,
  CONSTRAINT "ux_environment_permission_grant" UNIQUE ("environment_id", "role", "permission")
);

CREATE TABLE "project_invitations" (
  "id" BIGSERIAL PRIMARY KEY,
  "project_id" BIGINT NOT NULL,
  "role" VARCHAR(16) NOT NULL,
  "token_hash" BYTEA NOT NULL UNIQUE,
  "created_by" BIGINT NOT NULL,
  "expires_at" TIMESTAMPTZ NOT NULL,
  "accepted_by" BIGINT NULL,
  "accepted_at" TIMESTAMPTZ NULL,
  "revoked_at" TIMESTAMPTZ NULL,
  "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "ck_project_invitations_role" CHECK ("role" IN ('OWNER', 'MAINTAINER', 'MEMBER', 'GUEST'))
);
CREATE INDEX "ix_environment_permission_grants_environment" ON "environment_permission_grants" ("environment_id");



CREATE UNIQUE INDEX ux_users_email_lower ON users (lower(email));
CREATE UNIQUE INDEX ux_users_username_lower ON users (lower(username));

INSERT INTO application_settings (id, allow_self_registration) VALUES (1, FALSE);

CREATE UNIQUE INDEX ux_project_api_keys_secret_digest ON project_api_keys (secret_hash);
