package servergroup

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
)

const defaultTerminationTimeout = 10 * time.Second

type server struct {
	server Server
}

type Group struct {
	mu struct {
		sync.RWMutex
		servers []*server
	}

	termCh chan struct{}
	errChs []chan error
}

func (g *Group) Add(s Server) {
	svr := &server{server: s}

	g.mu.Lock()
	g.mu.servers = append(g.mu.servers, svr)
	g.mu.Unlock()
}

func (g *Group) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)

	g.mu.RLock()
	g.termCh = make(chan struct{}, len(g.mu.servers))
	g.errChs = make([]chan error, len(g.mu.servers))
	for i, s := range g.mu.servers {
		i := i
		s := s

		g.errChs[i] = make(chan error, 1)

		go func() {
			err := s.server.Start(ctx)
			g.termCh <- struct{}{}
			g.errChs[i] <- err
		}()
	}
	g.mu.RUnlock()

	var errMessageStop string
	var errMessageWait string

	select {
	case <-ctx.Done():
		errMessageStop = "failed to stop the servers due to: %w"
		errMessageWait = "server group failed to terminate the servers due to: %w"

	case <-g.termCh:
		errMessageStop = "failed to stop servers when the servers have been aborted due to: %w"
		errMessageWait = "server group aborted due to: %w"
	}

	cancel()

	// TODO: make timeout duration customizable
	termCtx, termCancel := context.WithTimeout(context.Background(), defaultTerminationTimeout)
	defer termCancel()

	termErrCh := make(chan error, 1)

	go func() {
		if err := g.stop(termCtx); err != nil {
			termErrCh <- fmt.Errorf(errMessageStop, err)
			return
		}

		if err := g.wait(); err != nil {
			termErrCh <- fmt.Errorf(errMessageWait, err)
			return
		}

		termErrCh <- nil
	}()

	select {
	case <-termCtx.Done():
		return errors.New("deadline exceeded for stopping the servers")
	case err := <-termErrCh:
		if err != nil {
			return fmt.Errorf("failed to terminate the servers: %w", err)
		}
		return nil
	}
}

func (g *Group) wait() error {
	eg := multierror.Group{}

	for _, errCh := range g.errChs {
		errCh := errCh
		eg.Go(func() error {
			return <-errCh
		})
	}

	return eg.Wait().ErrorOrNil()
}

func (g *Group) stop(ctx context.Context) error {
	g.mu.RLock()
	var stoppers []Stopper
	for _, s := range g.mu.servers {
		if st, ok := s.server.(Stopper); ok {
			stoppers = append(stoppers, st)
		}
	}
	g.mu.RUnlock()

	if len(stoppers) == 0 {
		return nil
	}

	var eg multierror.Group

	for _, s := range stoppers {
		s := s
		eg.Go(func() error {
			return s.Stop(ctx)
		})
	}

	if err := eg.Wait().ErrorOrNil(); err != nil {
		return fmt.Errorf("failed to stop the servers: %w", err)
	}

	return nil
}
