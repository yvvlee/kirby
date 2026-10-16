package model

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strings"
)

//go:embed sql/schema_mysql.sql sql/schema_pgsql.sql
var schemaSQL embed.FS

var requiredTables = []string{
	"application_secrets", "audit_logs", "config_enums", "configs", "environments", "import_records",
	"permissions", "project_api_keys", "projects", "refresh_tokens", "role_permissions",
	"roles", "snapshots", "structures", "user_environment_roles", "users",
}

// RequiredTables returns an independent schema inventory for startup checks.
//
// Parameters: none.
// Returns: the table names required by this server version.
// Throws: none.
func RequiredTables() []string {
	return append([]string(nil), requiredTables...)
}

// InitializeDatabase serializes first-start schema and credential creation so
// concurrent instances cannot commit different installation identities.
//
// Parameters: ctx controls startup; database owns the pool; postgres selects its dialect.
// Returns: an error if initialization or schema verification fails.
// Throws: none.
func InitializeDatabase(ctx context.Context, database *sql.DB, postgres bool) (err error) {
	connection, err := database.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve schema initialization connection: %w", err)
	}
	defer connection.Close()
	unlock, err := lockSchema(ctx, connection, postgres)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, unlock()) }()
	defer rollbackSchema(connection)
	return initializeSchema(ctx, connection, postgres)
}

func initializeSchema(ctx context.Context, connection *sql.Conn, postgres bool) error {
	schemaExpr, schemaFile := "DATABASE()", "sql/schema_mysql.sql"
	if postgres {
		schemaExpr, schemaFile = "current_schema()", "sql/schema_pgsql.sql"
	}
	var tableCount int
	query := "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = " + schemaExpr + " AND table_name = 'users'"
	if err := connection.QueryRowContext(ctx, query).Scan(&tableCount); err != nil {
		return fmt.Errorf("inspect database initialization state: %w", err)
	}
	if tableCount > 0 {
		return validateSchema(ctx, connection, postgres)
	}
	return executeSchema(ctx, connection, schemaFile, postgres)
}

func executeSchema(ctx context.Context, connection *sql.Conn, schemaFile string, postgres bool) error {
	schema, err := schemaSQL.ReadFile(schemaFile)
	if err != nil {
		return fmt.Errorf("read embedded database schema: %w", err)
	}
	for _, part := range strings.Split(string(schema), ";") {
		statement := schemaStatement(part)
		if statement == "" {
			continue
		}
		if err := executeSchemaStatement(ctx, connection, statement, postgres); err != nil {
			return err
		}
	}
	return validateSchema(ctx, connection, postgres)
}

func schemaStatement(part string) string {
	part = strings.TrimSpace(part)
	for strings.HasPrefix(part, "--") {
		_, rest, found := strings.Cut(part, "\n")
		if !found {
			return ""
		}
		part = strings.TrimSpace(rest)
	}
	return part
}

func executeSchemaStatement(ctx context.Context, connection *sql.Conn, statement string, postgres bool) error {
	// The seed script commits users, permissions, and installation keys together.
	if statement == "COMMIT" {
		if err := seedApplicationSecrets(ctx, connection, postgres); err != nil {
			return err
		}
	}
	if _, err := connection.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("execute database initialization statement: %w", err)
	}
	return nil
}

// ValidateSchema rejects incomplete initialization instead of serving requests
// against missing tables.
//
// Parameters: ctx controls inspection; database owns the pool; postgres selects its dialect.
// Returns: an error listing missing tables or a database read error.
// Throws: none.
func ValidateSchema(ctx context.Context, database *sql.DB, postgres bool) error {
	return validateSchema(ctx, database, postgres)
}

type schemaReader interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func validateSchema(ctx context.Context, reader schemaReader, postgres bool) error {
	schemaExpr := "DATABASE()"
	if postgres {
		schemaExpr = "current_schema()"
	}
	placeholders := schemaPlaceholders(postgres)
	query := "SELECT table_name FROM information_schema.tables WHERE table_schema = " + schemaExpr + " AND table_type = 'BASE TABLE' AND table_name IN (" + placeholders + ")"
	args := make([]any, len(requiredTables))
	for index, table := range requiredTables {
		args[index] = table
	}
	rows, err := reader.QueryContext(ctx, query, args...)
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
		return fmt.Errorf("database schema is incomplete; missing tables: %s", strings.Join(missing, ", "))
	}
	return nil
}

func schemaPlaceholders(postgres bool) string {
	values := make([]string, len(requiredTables))
	for index := range values {
		values[index] = "?"
		if postgres {
			values[index] = fmt.Sprintf("$%d", index+1)
		}
	}
	return strings.Join(values, ",")
}
