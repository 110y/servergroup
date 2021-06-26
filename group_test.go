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
	nopServer := servergroup.ServerFunc(func(ctx context.Context) error {
		wg.Done()
		<-ctx.Done()
		return nil
	})

	g.Add(nopServer)
	g.Add(nopServer)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		wg.Wait()
		cancel()
	}()

	if err := g.Start(ctx); err != nil {
		t.Errorf("failed: %s", err)
	}
}
