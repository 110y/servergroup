package servergroup

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrAlreadyStarted     = errors.New("already started")
	ErrTerminationTimeout = errors.New("termination timeout")
)

type Group struct {
	serversMu struct {
		sync.RWMutex
		servers []Server
	}

	startedMu struct {
		sync.RWMutex
		started bool
	}

	termCh chan struct{}
	errChs []chan error
}

func (g *Group) Add(s Server) {
	g.startedMu.RLock()
	if g.startedMu.started {
		return
	}
	g.startedMu.RUnlock()

	g.serversMu.Lock()
	g.serversMu.servers = append(g.serversMu.servers, s)
	g.serversMu.Unlock()
}

func (g *Group) Start(ctx context.Context, opts ...Option) error {
	g.startedMu.RLock()
	if g.startedMu.started {
		g.startedMu.RUnlock()
		return ErrAlreadyStarted
	}
	g.startedMu.RUnlock()

	g.startedMu.Lock()
	g.startedMu.started = true
	g.startedMu.Unlock()

	opt := newOption(opts...)

	ctx, cancel := context.WithCancel(ctx)

	g.serversMu.RLock()
	g.termCh = make(chan struct{}, len(g.serversMu.servers))
	g.errChs = make([]chan error, len(g.serversMu.servers))
	for i, s := range g.serversMu.servers {
		g.errChs[i] = make(chan error, 1)

		go func(i int, s Server) {
			err := s.Start(ctx)
			g.termCh <- struct{}{}
			g.errChs[i] <- err
		}(i, s)
	}
	g.serversMu.RUnlock()

	var aborted bool
	select {
	case <-ctx.Done():
	case <-g.termCh:
		aborted = true
	}

	cancel()

	termCtx, termCancel := context.WithTimeout(context.Background(), opt.terminationTimeout)
	defer termCancel()

	termErrCh := make(chan error, 1)

	go func() {
		if err := g.stop(termCtx); err != nil {
			if aborted {
				termErrCh <- fmt.Errorf("aborted: %w", err)
			} else {
				termErrCh <- fmt.Errorf("failed to stop servers: %w", err)
			}

			return
		}

		close(termErrCh)
	}()

	select {
	case <-termCtx.Done():
		return ErrTerminationTimeout
	case err, ok := <-termErrCh:
		if ok && err != nil {
			return err
		}
		return nil
	}
}

func (g *Group) stop(ctx context.Context) error {
	g.serversMu.RLock()
	var stoppers []Stopper
	for _, s := range g.serversMu.servers {
		if stopper, ok := s.(Stopper); ok {
			stoppers = append(stoppers, stopper)
		}
	}
	g.serversMu.RUnlock()

	var errMu struct {
		mu   sync.Mutex
		errs []error
	}

	var wg sync.WaitGroup

	for _, s := range stoppers {
		wg.Add(1)

		go func(s Stopper) {
			defer wg.Done()

			err := s.Stop(ctx)

			errMu.mu.Lock()
			errMu.errs = append(errMu.errs, err)
			errMu.mu.Unlock()
		}(s)
	}

	wg.Wait()

	for _, errCh := range g.errChs {
		wg.Add(1)

		go func(errCh chan error) {
			defer wg.Done()

			err := <-errCh

			errMu.mu.Lock()
			errMu.errs = append(errMu.errs, err)
			errMu.mu.Unlock()
		}(errCh)
	}

	wg.Wait()

	return joinErrors(errMu.errs...)
}
