package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/repository/base"
)

var ErrKeyConflict = errors.New("repository: resource key conflict")

func keyConflict(resource string) error {
	return fmt.Errorf("%w: %s", ErrKeyConflict, resource)
}

func lockConfigParent(ctx context.Context, tx *xorm.Session, environmentID, configID int64) error {
	var config struct {
		ID int64 `xorm:"id"`
	}
	return base.LockOne(ctx, tx, "config", `
SELECT c.id
FROM configs AS c
INNER JOIN projects AS p ON p.id = c.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND c.id = ? AND c.deleted_at IS NULL
LIMIT 1
FOR UPDATE`, []any{environmentID, configID}, &config)
}

func classifyKeyWriteError(resource string, err error) error {
	if err == nil {
		return nil
	}
	var mysqlError *mysql.MySQLError
	if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		return keyConflict(resource)
	}
	return err
}
