package drvregistry

import (
	"context"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/portmapperapi"
	"gotest.tools/v3/assert"
)

type fakePortMapper struct{}

func (f fakePortMapper) MapPorts(_ context.Context, _ []portmapperapi.PortBindingReq, _ portmapperapi.Firewaller) ([]portmapperapi.PortBinding, error) {
	return nil, nil
}

func (f fakePortMapper) UnmapPorts(_ context.Context, _ []portmapperapi.PortBinding, _ portmapperapi.Firewaller) error {
	return nil
}

func TestRegisterPortMappers(t *testing.T) {
	t.Run("register port mapper", func(t *testing.T) {
		var pms PortMappers

		pm := fakePortMapper{}
		err := pms.Register("test", pm)
		assert.NilError(t, err)
	})

	t.Run("empty name", func(t *testing.T) {
		var pms PortMappers

		err := pms.Register("", nil)
		assert.ErrorContains(t, err, "portmapper name cannot be empty")
	})

	t.Run("duplicate port mapper", func(t *testing.T) {
		var pms PortMappers

		err := pms.Register("test", nil)
		assert.NilError(t, err)

		err = pms.Register("test", nil)
		assert.ErrorContains(t, err, "portmapper already registered")
	})
}

func TestGetPortMapper(t *testing.T) {
	t.Run("get existing port mapper", func(t *testing.T) {
		var pms PortMappers

		pm := fakePortMapper{}
		err := pms.Register("test", pm)
		assert.NilError(t, err)

		retrieved, err := pms.Get("test")
		assert.NilError(t, err)
		assert.Equal(t, retrieved, pm)
	})

	t.Run("get nonexistent port mapper", func(t *testing.T) {
		var pms PortMappers

		_, err := pms.Get("nonexistent")
		assert.ErrorContains(t, err, "portmapper nonexistent not found")
	})
}
