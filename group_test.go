package servergroup_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/110y/servergroup"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestGroup(t *testing.T) {
	t.Parallel()

	var g servergroup.Group

	var wg sync.WaitGroup
	wg.Add(2)

	var startedMu struct {
		sync.Mutex
		count int
	}

	start := func(ctx context.Context) error {
		startedMu.Lock()
		startedMu.count++
		startedMu.Unlock()

		wg.Done()
		<-ctx.Done()
		return nil
	}

	stop := func(_ context.Context) error {
		return nil
	}

	server := &stoppableServer{
		start: start,
		stop:  stop,
	}

	serverFunc := servergroup.ServerFunc(start)

	g.Add(server)
	g.Add(serverFunc)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		wg.Wait()
		cancel()
	}()

	if err := g.Start(ctx); err != nil {
		t.Errorf("failed: %s", err)
		return
	}

	if startedMu.count != 2 {
		t.Errorf("want 2, got %d", startedMu.count)
	}
}

func TestGroupErrorAborted(t *testing.T) {
	t.Parallel()

	var g servergroup.Group

	g.Add(servergroup.ServerFunc(func(_ context.Context) error {
		return errors.New("error!")
	}))

	ctx := context.Background()

	if err := g.Start(ctx); err != nil {
		if err.Error() != "aborted: error!" {
			t.Errorf("want an aborted error, got `%v`", err)
		}
	}
}

func TestGroupErrorStop(t *testing.T) {
	t.Parallel()

	startedCh := make(chan struct{})

	server := &stoppableServer{
		start: func(ctx context.Context) error {
			close(startedCh)
			<-ctx.Done()
			return nil
		},
		stop: func(_ context.Context) error {
			return errors.New("failed to stop server")
		},
	}

	var g servergroup.Group

	g.Add(server)

	ctx, cancel := context.WithCancel(context.Background())

	stoppedCh := make(chan error, 1)
	go func() {
		stoppedCh <- g.Start(ctx)
	}()

	<-startedCh

	cancel()

	if err := <-stoppedCh; err != nil {
		if err.Error() != "failed to stop servers: failed to stop server" {
			t.Errorf("want a stop error, got `%v`", err)
		}
	}
}

func TestGroupErrorTerminationTimeout(t *testing.T) {
	t.Parallel()

	startedCh := make(chan struct{})

	server := &stoppableServer{
		start: func(ctx context.Context) error {
			close(startedCh)
			<-ctx.Done()
			return nil
		},
		stop: func(_ context.Context) error {
			time.Sleep(2 * time.Millisecond)
			return nil
		},
	}

	var g servergroup.Group

	g.Add(server)

	ctx, cancel := context.WithCancel(context.Background())

	stoppedCh := make(chan error, 1)
	go func() {
		stoppedCh <- g.Start(ctx, servergroup.WithTerminationTimeout(1*time.Millisecond))
	}()

	<-startedCh

	cancel()

	if err := <-stoppedCh; err != nil {
		if err != servergroup.ErrTerminationTimeout {
			t.Errorf("want a termination timeout error, got `%v`", err)
		}
	}
}

func TestGroupStartTwice(t *testing.T) {
	t.Parallel()

	var g servergroup.Group

	startedCh := make(chan struct{})

	g.Add(servergroup.ServerFunc(func(ctx context.Context) error {
		close(startedCh)
		<-ctx.Done()
		return nil
	}))

	ctx, cancel := context.WithCancel(context.Background())

	stoppedCh := make(chan struct{})
	go func() {
		if err := g.Start(ctx); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		close(stoppedCh)
	}()

	<-startedCh

	// This call of Add ensures that it is safe to call Add after Start.
	g.Add(servergroup.ServerFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}))

	got := g.Start(ctx)
	if !errors.Is(got, servergroup.ErrAlreadyStarted) {
		t.Errorf("want ErrAlreadyStarted, but got: %s", got)
	}

	cancel()
	<-stoppedCh
}

var (
	_ servergroup.Server  = (*stoppableServer)(nil)
	_ servergroup.Stopper = (*stoppableServer)(nil)
)

type stoppableServer struct {
	start func(context.Context) error
	stop  func(context.Context) error
}

func (s *stoppableServer) Start(ctx context.Context) error {
	return s.start(ctx)
}

func (s *stoppableServer) Stop(ctx context.Context) error {
	return s.stop(ctx)
}
