package server

import "context"

type serverTransactions struct{}

func (serverTransactions) TX(ctx context.Context, operation func(context.Context) error) error {
	return operation(ctx)
}
