package logic

import "context"

// directTransactions explicitly opts fake repositories into non-transactional tests.
// Database transaction behavior is covered with the real lorm engine in infra.
type directTransactions struct{}

func (directTransactions) TX(ctx context.Context, operation func(context.Context) error) error {
	return operation(ctx)
}

type transactionFunc func(context.Context, func(context.Context) error) error

func (run transactionFunc) TX(ctx context.Context, operation func(context.Context) error) error {
	return run(ctx, operation)
}
