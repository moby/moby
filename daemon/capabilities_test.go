package daemon

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"
)

type mockDaemon struct {
	usesSnapshotter func() bool
}

func (m *mockDaemon) UsesSnapshotter() bool {
	return m.usesSnapshotter()
}

func TestGetCapabilities(t *testing.T) {
	t.Run("snapshotter enabled", func(t *testing.T) {
		manager := capabilitiesManager{}
		mockDaemon := &mockDaemon{}
		mockDaemon.usesSnapshotter = func() bool {
			return true
		}

		capabilities, err := manager.getCapabilities(context.Background(), mockDaemon)
		assert.NilError(t, err)

		assert.Equal(t, true, capabilities.RegistryClientAuth)
	})

	t.Run("snapshotter disabled", func(t *testing.T) {
		manager := capabilitiesManager{}
		mockDaemon := &mockDaemon{}
		mockDaemon.usesSnapshotter = func() bool {
			return false
		}

		capabilities, err := manager.getCapabilities(context.Background(), mockDaemon)
		assert.NilError(t, err)

		assert.Equal(t, false, capabilities.RegistryClientAuth)
	})
}

func TestCache(t *testing.T) {
	t.Run("first call", func(t *testing.T) {
		manager := capabilitiesManager{}
		mockDaemon := &mockDaemon{}
		var called int
		mockDaemon.usesSnapshotter = func() bool {
			called++
			return true
		}

		_, _ = manager.getCapabilities(context.Background(), mockDaemon)

		assert.Equal(t, 1, called)
	})

	t.Run("no invalidate", func(t *testing.T) {
		manager := capabilitiesManager{}
		mockDaemon := &mockDaemon{}
		var called int
		mockDaemon.usesSnapshotter = func() bool {
			called++
			return true
		}

		_, _ = manager.getCapabilities(context.Background(), mockDaemon)

		assert.Equal(t, 1, called)

		_, _ = manager.getCapabilities(context.Background(), mockDaemon)

		assert.Equal(t, 1, called)
	})

	t.Run("invalidate fetches again", func(t *testing.T) {
		manager := capabilitiesManager{}
		mockDaemon := &mockDaemon{}
		var called int
		mockDaemon.usesSnapshotter = func() bool {
			called++
			return true
		}
		ctx := context.Background()

		_, _ = manager.getCapabilities(ctx, mockDaemon)
		assert.Equal(t, 1, called)

		_, _ = manager.getCapabilities(ctx, mockDaemon)
		assert.Equal(t, 1, called)

		manager.invalidateCache(ctx)

		_, _ = manager.getCapabilities(ctx, mockDaemon)
		assert.Equal(t, 2, called)
	})
}
