package volumestore

import (
	"github.com/docker/docker/component"
	"github.com/docker/docker/volume/store"
	"golang.org/x/net/context"
)

const name = "volumestore"

func Set(s *store.VolumeStore) (cancel func(), err error) {
	return component.Register(name, s)
}

func Get(ctx context.Context) (*store.VolumeStore, error) {
	c := component.Wait(ctx, name)
	if c == nil {
		return nil, ctx.Err()
	}

	// This could panic... but I think this is ok.
	// This should never be anything else
	return c.(*store.VolumeStore), nil
}
