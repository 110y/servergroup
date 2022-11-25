package servergroup_test

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"golang.org/x/sys/unix"

	"github.com/110y/servergroup"
)

// Your servers must implement servergroup.Server interface:
//
//	type Server interface {
//	   Start(context.Context) error
//	}
var (
	_ servergroup.Server = (*server1)(nil)
	_ servergroup.Server = (*server2)(nil)
)

type server1 struct{}

func (s *server1) Start(ctx context.Context) error {
	// Your own server logic here. (e.g. Call http.Server.Serve)
	<-ctx.Done()
	return nil
}

type server2 struct{}

func (s *server2) Start(ctx context.Context) error {
	// Your own server logic here. (e.g. Call http.Server.Serve)
	<-ctx.Done()
	return nil
}

func main() {
}

func Example() {
	var group servergroup.Group

	// Add your servers to the Group by calling Group.Add.
	group.Add(&server1{})
	group.Add(&server2{})

	ctx, cancel := signal.NotifyContext(context.Background(), unix.SIGTERM)
	defer cancel()

	// Group.Start calls Start methods of all added servers concurrently.
	// When the context is canceled, Group.Start waits for all servers to stop and returns an error if any of them failed.
	if err := group.Start(ctx); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}
