package capabilities

import (
	"context"
	"errors"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

type mockDaemon struct {
	usesSnapshotter func() bool
}

func (m *mockDaemon) UsesSnapshotter() bool {
	return m.usesSnapshotter()
}

func TestGetCapabilities(t *testing.T) {
	t.Run("V1 returns expected values", func(t *testing.T) {
		t.Run("if snapshotter enabled", func(t *testing.T) {
			manager := NewManager()
			mockDaemon := &mockDaemon{}
			mockDaemon.usesSnapshotter = func() bool {
				return true
			}

			caps, err := manager.GetCapabilities(context.Background(), mockDaemon, 1)
			assert.NilError(t, err)

			v1Caps, ok := caps.(capabilitiesV1)
			assert.Check(t, ok)
			assert.Equal(t, true, v1Caps.RegistryClientAuth, "registry-client-auth should be true if snapshotter enabled")
		})

		t.Run("if snapshotter disabled", func(t *testing.T) {
			manager := NewManager()
			mockDaemon := &mockDaemon{}
			mockDaemon.usesSnapshotter = func() bool {
				return false
			}

			caps, err := manager.GetCapabilities(context.Background(), mockDaemon, 1)
			assert.NilError(t, err)

			v1Caps, ok := caps.(capabilitiesV1)
			assert.Check(t, ok)
			assert.Equal(t, false, v1Caps.RegistryClientAuth, "registry-client-auth should be false if snapshotter disabled ")
		})
	})
}

func TestVersionNegotiation(t *testing.T) {
	t.Run("assume latest version when requested version is", func(t *testing.T) {
		testCases := []struct {
			doc              string
			requestedVersion int
		}{
			{
				doc:              "same as the daemon's",
				requestedVersion: int(CurrentVersion),
			},
			{
				doc:              "newer than the daemon's",
				requestedVersion: 1000,
			},
			{
				doc:              "unknown/looks malformed",
				requestedVersion: -1,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.doc, func(t *testing.T) {
				manager := NewManager()
				mockDaemon := &mockDaemon{}
				mockDaemon.usesSnapshotter = func() bool {
					return true
				}

				capabilities, err := manager.GetCapabilities(context.Background(), mockDaemon, tc.requestedVersion)
				assert.NilError(t, err)

				assert.Equal(t, capabilities.Version(), CurrentVersion)
			})
		}
	})
}

func TestManagerInvalidateCache(t *testing.T) {
	t.Run("if the manager does not have cache", func(t *testing.T) {
		t.Run("invalidate does not cause cache to be fetched multiple times", func(t *testing.T) {
			manager := NewManager()
			mockDaemon := &mockDaemon{}
			var called int
			mockDaemon.usesSnapshotter = func() bool {
				called++
				return true
			}
			ctx := context.Background()

			assert.Equal(t, 0, called)
			manager.InvalidateCache(ctx)
			assert.Equal(t, 0, called)

			_, _ = manager.GetCapabilities(ctx, mockDaemon, int(CurrentVersion))
			assert.Equal(t, 1, called)
			_, _ = manager.GetCapabilities(ctx, mockDaemon, int(CurrentVersion))
			assert.Equal(t, 1, called)
		})
	})

	t.Run("if the manager already has cache", func(t *testing.T) {
		t.Run("invalidate causes cache to be fetched again", func(t *testing.T) {
			manager := NewManager()
			manager.cache.v1 = capabilitiesV1{}
			manager.cacheReady.Store(true)
			mockDaemon := &mockDaemon{}
			var called int
			mockDaemon.usesSnapshotter = func() bool {
				called++
				return true
			}
			ctx := context.Background()

			_, _ = manager.GetCapabilities(ctx, mockDaemon, int(CurrentVersion))
			assert.Equal(t, 0, called)

			manager.InvalidateCache(ctx)

			_, _ = manager.GetCapabilities(ctx, mockDaemon, int(CurrentVersion))
			assert.Equal(t, 1, called)
		})

		t.Run("invalidate causes cache to be fetched again multiple times", func(t *testing.T) {
			manager := NewManager()
			manager.cache.v1 = capabilitiesV1{}
			manager.cacheReady.Store(true)
			mockDaemon := &mockDaemon{}
			var called int
			mockDaemon.usesSnapshotter = func() bool {
				called++
				return true
			}
			ctx := context.Background()

			_, _ = manager.GetCapabilities(ctx, mockDaemon, int(CurrentVersion))
			assert.Equal(t, 0, called)

			manager.InvalidateCache(ctx)

			_, _ = manager.GetCapabilities(ctx, mockDaemon, int(CurrentVersion))
			assert.Equal(t, 1, called)

			manager.InvalidateCache(ctx)

			_, _ = manager.GetCapabilities(ctx, mockDaemon, int(CurrentVersion))
			assert.Equal(t, 2, called)

			manager.InvalidateCache(ctx)

			_, _ = manager.GetCapabilities(ctx, mockDaemon, int(CurrentVersion))
			assert.Equal(t, 3, called)
		})
	})

	// TODO(laurazard): can probably clean up this test
	t.Run("doesn't lock if there are many concurrent GetCapabilities calls", func(t *testing.T) {
		manager := NewManager()
		manager.cache = capabilitiesCache{
			v1: capabilitiesV1{
				CapabilitiesBase: CapabilitiesBase{
					CapabilitiesVersion: 1,
				},
				RegistryClientAuth: false,
			},
		}
		manager.cacheReady.Store(true)
		mockDaemon := &mockDaemon{}
		mockDaemon.usesSnapshotter = func() bool {
			time.Sleep(2 * time.Second)
			return true
		}

		ctx := context.Background()
		stop := make(chan struct{})
		capC := make(chan capabilitiesV1)
		errC := make(chan error)
		// keep launching goroutines to call GetCapabilities until we
		// receive on stop
		go func() {
			for {
				select {
				case <-stop:
					return
				default:
				}

				go func() {
					c, err := manager.GetCapabilities(ctx, mockDaemon, 1)
					if err != nil {
						errC <- err
						return
					}
					v1caps, ok := c.(capabilitiesV1)
					if !ok {
						errC <- errors.New("failed to cast c to capabilitiesV1")
						return
					}
					capC <- v1caps
				}()
				time.Sleep(time.Millisecond)
			}
		}()

	A:
		for {
			select {
			case c := <-capC:
				assert.Check(t, !c.RegistryClientAuth, "c.RegistryClientAuth should be false before cache invalidation")
				break A
			case err := <-errC:
				t.Fatal(err)
			}
		}

		done := make(chan struct{})
		go func() {
			manager.InvalidateCache(ctx)
			done <- struct{}{}
		}()

		select {
		case <-done:
		case <-time.After(1 * time.Millisecond):
			t.Fatal("took too long!")
		}

	B:
		for {
			select {
			case caps := <-capC:
				if caps.RegistryClientAuth == true {
					break B
				}
			case err := <-errC:
				t.Fatal(err)
			}
		}
	})
}

func TestManagerConcurrency(t *testing.T) {
	t.Run("many concurrent calls while cache dirty only results in single refresh", func(t *testing.T) {
		manager := NewManager()
		mockDaemon := &mockDaemon{}
		var timesCalled int
		mockDaemon.usesSnapshotter = func() bool {
			timesCalled++
			time.Sleep(2 * time.Second)
			return true
		}

		ctx := context.Background()
		concurrentCalls := 10000
		done := make(chan error, 5)
		for i := 0; i < concurrentCalls; i++ {
			go func() {
				_, err := manager.GetCapabilities(ctx, mockDaemon, 1)
				done <- err
			}()
		}

		for i := 0; i < concurrentCalls; i++ {
			err := <-done
			assert.NilError(t, err)
		}

		assert.Equal(t, 1, timesCalled)
	})

	t.Run("cache invalidation during concurrent calls to GetCapabilities", func(t *testing.T) {
		manager := NewManager()
		manager.cache = capabilitiesCache{
			v1: capabilitiesV1{
				CapabilitiesBase: CapabilitiesBase{
					CapabilitiesVersion: 1,
				},
				RegistryClientAuth: false,
			},
		}
		manager.cacheReady.Store(true)
		mockDaemon := &mockDaemon{}
		mockDaemon.usesSnapshotter = func() bool {
			time.Sleep(2 * time.Second)
			return true
		}

		ctx := context.Background()
		stop := make(chan struct{})
		capC := make(chan capabilitiesV1)
		errC := make(chan error)
		// keep launching goroutines to call GetCapabilities until we
		// receive on stop
		go func() {
			for {
				select {
				case <-stop:
					return
				default:
				}

				go func() {
					caps, err := manager.GetCapabilities(ctx, mockDaemon, 1)
					if err != nil {
						errC <- errors.New("failed to cast caps to capabilitiesV1")
						return
					}
					v1caps, ok := caps.(capabilitiesV1)
					if !ok {
						errC <- errors.New("failed to cast caps to capabilitiesV1")
						return
					}
					capC <- v1caps
				}()
				time.Sleep(time.Millisecond)
			}
		}()

	A:
		for {
			select {
			case caps := <-capC:
				assert.Check(t, !caps.RegistryClientAuth, "caps.RegistryClientAuth should be false before cache invalidation")
				break A
			case err := <-errC:
				t.Fatal(err)
			}
		}

		manager.InvalidateCache(ctx)

	B:
		for {
			select {
			case caps := <-capC:
				if caps.RegistryClientAuth == true {
					break B
				}
			case err := <-errC:
				t.Fatal(err)
			}
		}

		stop <- struct{}{}
	})
}
