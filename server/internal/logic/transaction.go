package logic

import "context"

// TransactionRunner separates atomic business operations from the database driver.
// Production binds this interface directly to the shared lorm engine.
type TransactionRunner interface {
	TX(ctx context.Context, operation func(context.Context) error) error
}
