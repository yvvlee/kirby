package model

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"
)

const (
	mysqlSchemaLock   = "SELECT GET_LOCK(SHA2(CONCAT(DATABASE(), ':kirby:init'), 256), 60)"
	mysqlSchemaUnlock = "SELECT RELEASE_LOCK(SHA2(CONCAT(DATABASE(), ':kirby:init'), 256))"
	pgSchemaLock      = "SELECT pg_advisory_lock(hashtext(current_schema()), hashtext('kirby:init'))"
	pgSchemaUnlock    = "SELECT pg_advisory_unlock(hashtext(current_schema()), hashtext('kirby:init'))"
)

func lockSchema(ctx context.Context, connection *sql.Conn, postgres bool) (func() error, error) {
	if err := acquireSchemaLock(ctx, connection, postgres); err != nil {
		// A canceled query may have acquired a session lock before cancellation.
		discardSchemaConnection(connection)
		return nil, fmt.Errorf("lock database initialization: %w", err)
	}
	return func() error {
		return releaseSchemaLock(connection, postgres)
	}, nil
}

func acquireSchemaLock(ctx context.Context, connection *sql.Conn, postgres bool) error {
	if postgres {
		_, err := connection.ExecContext(ctx, pgSchemaLock)
		return err
	}
	var acquired sql.NullInt64
	if err := connection.QueryRowContext(ctx, mysqlSchemaLock).Scan(&acquired); err != nil {
		return err
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return fmt.Errorf("database initialization lock was not acquired")
	}
	return nil
}

func releaseSchemaLock(connection *sql.Conn, postgres bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := mysqlSchemaUnlock
	if postgres {
		query = pgSchemaUnlock
	}
	if _, err := connection.ExecContext(ctx, query); err != nil {
		discardSchemaConnection(connection)
		return fmt.Errorf("unlock database initialization: %w", err)
	}
	return nil
}

func rollbackSchema(connection *sql.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// A failed seed must not leave its transaction open on a pooled connection.
	if _, err := connection.ExecContext(ctx, "ROLLBACK"); err != nil {
		discardSchemaConnection(connection)
	}
}

func discardSchemaConnection(connection *sql.Conn) {
	_ = connection.Raw(func(any) error { return driver.ErrBadConn })
}
