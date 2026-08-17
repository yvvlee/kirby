package base

import (
	"context"
	"database/sql"
	"fmt"

	"xorm.io/xorm"
)

func FindOne(ctx context.Context, engine *xorm.Engine, resource, query string, args []any, destination any) error {
	if err := ValidateEngine(engine == nil); err != nil {
		return err
	}
	found, err := engine.Context(ctx).SQL(query, args...).Get(destination)
	if err != nil {
		return Wrap("find "+resource, err)
	}
	if !found {
		return Missing(resource)
	}
	return nil
}

func FindAll(ctx context.Context, engine *xorm.Engine, resource, query string, args []any, destination any) error {
	if err := ValidateEngine(engine == nil); err != nil {
		return err
	}
	if err := engine.Context(ctx).SQL(query, args...).Find(destination); err != nil {
		return Wrap("list "+resource, err)
	}
	return nil
}

func Count(ctx context.Context, engine *xorm.Engine, resource, query string, args ...any) (int64, error) {
	var row struct {
		Total int64 `xorm:"total"`
	}
	if err := FindOne(ctx, engine, resource+" count", query, args, &row); err != nil {
		return 0, err
	}
	return row.Total, nil
}

// LockOne executes a SELECT ... FOR UPDATE on an existing transaction.
// Requiring the transaction session in the signature prevents a lock from
// being accidentally released before the protected operation runs.
func LockOne(ctx context.Context, tx *xorm.Session, resource, query string, args []any, destination any) error {
	if tx == nil {
		return InvalidArgument("transaction session is nil")
	}
	found, err := tx.Context(ctx).SQL(query, args...).Get(destination)
	if err != nil {
		return Wrap("lock "+resource, err)
	}
	if !found {
		return Missing(resource)
	}
	return nil
}

func Execute(ctx context.Context, engine *xorm.Engine, resource, query string, args ...any) (sql.Result, error) {
	if err := ValidateEngine(engine == nil); err != nil {
		return nil, err
	}
	statement := append([]any{query}, args...)
	result, err := engine.Context(ctx).Exec(statement...)
	if err != nil {
		return nil, Wrap("write "+resource, err)
	}
	if err := RequireAffected(resource, result); err != nil {
		return nil, err
	}
	return result, nil
}

func ExecuteTx(ctx context.Context, tx *xorm.Session, resource, query string, args ...any) (sql.Result, error) {
	if tx == nil {
		return nil, InvalidArgument("transaction session is nil")
	}
	statement := append([]any{query}, args...)
	result, err := tx.Context(ctx).Exec(statement...)
	if err != nil {
		return nil, Wrap("write "+resource, err)
	}
	if err := RequireAffected(resource, result); err != nil {
		return nil, err
	}
	return result, nil
}

func RequireAffected(resource string, result sql.Result) error {
	if result == nil {
		return fmt.Errorf("%w: %s returned a nil result", ErrNoRowsAffected, resource)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return Wrap("read affected rows for "+resource, err)
	}
	if rows == 0 {
		return Unchanged(resource)
	}
	return nil
}
