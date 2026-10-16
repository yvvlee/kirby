package service

import "context"

type serviceTransactions struct{}

func (serviceTransactions) TX(ctx context.Context, operation func(context.Context) error) error {
	return operation(ctx)
}
