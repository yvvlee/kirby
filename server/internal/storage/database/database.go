// Package database owns MySQL connection and transaction lifecycle.
package database

import (
	"context"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/config"
)

// Open connects to the existing schema. It never creates or alters tables.
func Open(ctx context.Context, cfg config.MySQLConfig) (*xorm.Engine, error) {
	engine, err := xorm.NewEngine("mysql", cfg.DSN.Value())
	if err != nil {
		return nil, fmt.Errorf("create MySQL engine: %w", err)
	}
	engine.SetMaxOpenConns(cfg.MaxOpenConns)
	engine.SetMaxIdleConns(cfg.MaxIdleConns)
	engine.SetConnMaxLifetime(cfg.ConnMaxLifetime.Duration)
	engine.ShowSQL(false)

	if err := engine.PingContext(ctx); err != nil {
		_ = engine.Close()
		return nil, fmt.Errorf("connect to MySQL: %w", err)
	}
	return engine, nil
}
