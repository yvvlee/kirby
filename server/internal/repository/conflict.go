package repository

import (
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrKeyConflict = errors.New("repository: resource key conflict")

func keyConflict(resource string) error {
	return fmt.Errorf("%w: %s", ErrKeyConflict, resource)
}

func classifyKeyWriteError(resource string, err error) error {
	if err == nil {
		return nil
	}
	if isDuplicateKeyError(err) {
		return keyConflict(resource)
	}
	return WrapDatabaseError(err)
}

func isDuplicateKeyError(err error) bool {
	var mysqlError *mysql.MySQLError
	if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		return true
	}
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
