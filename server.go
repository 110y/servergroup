package servergroup

import (
	"context"
)

// Server represents a server that can be added to a Group.
type Server interface {
	// Start starts the server and should block until the context is done.
	Start(context.Context) error
}

// Stopper represents a server that has a special logic to stop.
// If the server added to a Group implements this interface, Stop method will be called when the context passed to Group.Start is done.
type Stopper interface {
	// Stop stops the server.
	Stop(context.Context) error
}

var _ Server = (ServerFunc)(nil)

// ServerFunc is a function that implements Server interface.
// It can be used to wrap a function as a Server like below:
//
//	group.Add(servergroup.ServerFunc(func(ctx context.Context) error {
//	   // do something
//	   <-ctx.Done()
//	   return nil
//	}))
type ServerFunc func(ctx context.Context) error

// Start calls f(ctx).
func (f ServerFunc) Start(ctx context.Context) error {
	return f(ctx)
}
