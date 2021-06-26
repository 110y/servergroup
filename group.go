package servergroup

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
)

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

	select {
	case <-ctx.Done():
		cancel()
		if err := g.wait(); err != nil {
			return fmt.Errorf("server group failed to terminate servers due to: %w", err)
		}
		return nil

	case <-g.termCh:
		cancel()
		return fmt.Errorf("server group aborted due to: %w", g.wait())
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
