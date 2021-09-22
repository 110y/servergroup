package servergroup_test

import (
	"context"
	"sync"
	"testing"

	"github.com/110y/servergroup"
)

func TestGroup(t *testing.T) {
	t.Parallel()

	var g servergroup.Group

	var wg sync.WaitGroup
	wg.Add(2)

	start := func(ctx context.Context) error {
		wg.Done()
		<-ctx.Done()
		return nil
	}

	nopServer := servergroup.ServerFunc(start)

	st := &stoppableServer{
		start: start,
	}

	g.Add(nopServer)
	g.Add(st)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		wg.Wait()
		cancel()
	}()

	if err := g.Start(ctx); err != nil {
		t.Errorf("failed: %s", err)
		return
	}

	if st.stopped != true {
		t.Error("failed to stop")
	}
}

var (
	_ servergroup.Server  = (*stoppableServer)(nil)
	_ servergroup.Stopper = (*stoppableServer)(nil)
)

type stoppableServer struct {
	start   func(context.Context) error
	stopped bool
}

func (s *stoppableServer) Start(ctx context.Context) error {
	return s.start(ctx)
}

func (s *stoppableServer) Stop(ctx context.Context) error {
	s.stopped = true
	return nil
}
