package database

import (
	"context"
	"errors"
	"fmt"

	"xorm.io/xorm"
)

// WithTx executes fn in a transaction. Business errors and panics explicitly
// roll back before being returned or rethrown.
func WithTx(ctx context.Context, engine *xorm.Engine, fn func(*xorm.Session) error) (err error) {
	if engine == nil {
		return fmt.Errorf("database engine is nil")
	}
	if fn == nil {
		return fmt.Errorf("transaction function is nil")
	}

	session := engine.NewSession()
	defer session.Close()
	session = session.Context(ctx)
	if err := session.Begin(); err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			_ = session.Rollback()
			panic(recovered)
		}
	}()

	if err := fn(session); err != nil {
		if rollbackErr := session.Rollback(); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}
		return err
	}
	if err := session.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
