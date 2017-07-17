package containerstore

import (
	"github.com/docker/docker/component"
	"github.com/docker/docker/container"
	"golang.org/x/net/context"
)

const name = "containerstore"

func Set(s container.Store) (cancel func(), err error) {
	return component.Register(name, s)
}

func Get(ctx context.Context) (container.Store, error) {
	c := component.Wait(ctx, name)
	if c == nil {
		return nil, ctx.Err()
	}

	// This could panic... but I think this is ok.
	// This should never be anything else
	return c.(container.Store), nil
}