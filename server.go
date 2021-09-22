package servergroup

import (
	"context"
)

type Server interface {
	Start(context.Context) error
}

type Stopper interface {
	Stop(context.Context) error
}

var _ Server = (ServerFunc)(nil)

type ServerFunc func(ctx context.Context) error

func (f ServerFunc) Start(ctx context.Context) error {
	return f(ctx)
}
