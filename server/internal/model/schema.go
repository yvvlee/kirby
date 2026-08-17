package model

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"xorm.io/xorm"
)

var requiredTables = []string{
	"audit_logs",
	"config_enums",
	"configs",
	"environments",
	"import_records",
	"permissions",
	"project_api_keys",
	"projects",
	"refresh_tokens",
	"role_permissions",
	"roles",
	"snapshots",
	"structures",
	"user_environment_roles",
	"users",
}

// RequiredTables returns the fixed schema inventory. The returned slice is a
// copy so callers cannot mutate the startup contract.
func RequiredTables() []string {
	return append([]string(nil), requiredTables...)
}

// ValidateSchema verifies that the manually installed schema is complete.
// Server startup must call this after connecting to MySQL and before becoming
// ready. It only reads information_schema and never creates or alters tables.
func ValidateSchema(ctx context.Context, engine *xorm.Engine) error {
	if engine == nil {
		return fmt.Errorf("database engine is nil")
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(requiredTables)), ",")
	query := "SELECT table_name FROM information_schema.tables " +
		"WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' AND table_name IN (" + placeholders + ")"
	args := make([]any, len(requiredTables))
	for index, table := range requiredTables {
		args[index] = table
	}

	rows, err := engine.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("inspect database schema: %w", err)
	}
	defer rows.Close()

	found := make(map[string]struct{}, len(requiredTables))
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return fmt.Errorf("scan database schema: %w", err)
		}
		found[table] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read database schema: %w", err)
	}

	missing := make([]string, 0)
	for _, table := range requiredTables {
		if _, ok := found[table]; !ok {
			missing = append(missing, table)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("database schema is incomplete; run deploy/schema.sql manually; missing tables: %s", strings.Join(missing, ", "))
	}
	return nil
}
