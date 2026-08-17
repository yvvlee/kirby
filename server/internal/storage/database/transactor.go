package database

import (
	"context"
	"fmt"

	"xorm.io/xorm"
)

// Transactor is the single transaction boundary used by business logic.
type Transactor interface {
	WithTx(context.Context, func(*xorm.Session) error) error
}

type EngineTransactor struct{ engine *xorm.Engine }

func NewTransactor(engine *xorm.Engine) (*EngineTransactor, error) {
	if engine == nil {
		return nil, fmt.Errorf("database engine is nil")
	}
	return &EngineTransactor{engine: engine}, nil
}

func (t *EngineTransactor) WithTx(ctx context.Context, operation func(*xorm.Session) error) error {
	if t == nil {
		return fmt.Errorf("database transactor is nil")
	}
	return WithTx(ctx, t.engine, operation)
}

var _ Transactor = (*EngineTransactor)(nil)
