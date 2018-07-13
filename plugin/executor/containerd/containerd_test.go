package containerd

import (
	"context"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/libcontainerd"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"gotest.tools/assert"
)

func TestLifeCycle(t *testing.T) {
	t.Parallel()

	mock := newMockClient()
	exec, cleanup := setupTest(t, mock, mock)
	defer cleanup()

	id := "test-create"
	mock.simulateStartError(true, id)
	err := exec.Create(id, specs.Spec{}, nil, nil)
	assert.Assert(t, err != nil)
	mock.simulateStartError(false, id)

	err = exec.Create(id, specs.Spec{}, nil, nil)
	assert.Assert(t, err)
	running, _ := exec.IsRunning(id)
	assert.Assert(t, running)

	// create with the same ID
	err = exec.Create(id, specs.Spec{}, nil, nil)
	assert.Assert(t, err != nil)

	mock.HandleExitEvent(id) // simulate a plugin that exits

	err = exec.Create(id, specs.Spec{}, nil, nil)
	assert.Assert(t, err)
}

func setupTest(t *testing.T, client Client, eh ExitHandler) (*Executor, func()) {
	rootDir, err := ioutil.TempDir("", "test-daemon")
	assert.Assert(t, err)
	assert.Assert(t, client != nil)
	assert.Assert(t, eh != nil)

	return &Executor{
			rootDir:     rootDir,
			client:      client,
			exitHandler: eh,
		}, func() {
			assert.Assert(t, os.RemoveAll(rootDir))
		}
}

type mockClient struct {
	mu           sync.Mutex
	containers   map[string]bool
	errorOnStart map[string]bool
}

func newMockClient() *mockClient {
	return &mockClient{
		containers:   make(map[string]bool),
		errorOnStart: make(map[string]bool),
	}
}

func (c *mockClient) Create(ctx context.Context, id string, _ *specs.Spec, _ interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.containers[id]; ok {
		return errors.New("exists")
	}

	c.containers[id] = false
	return nil
}

func (c *mockClient) Restore(ctx context.Context, id string, attachStdio libcontainerd.StdioCallback) (alive bool, pid int, err error) {
	return false, 0, nil
}

func (c *mockClient) Status(ctx context.Context, id string) (libcontainerd.Status, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	running, ok := c.containers[id]
	if !ok {
		return libcontainerd.StatusUnknown, errors.New("not found")
	}
	if running {
		return libcontainerd.StatusRunning, nil
	}
	return libcontainerd.StatusStopped, nil
}

func (c *mockClient) Delete(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.containers, id)
	return nil
}

func (c *mockClient) DeleteTask(ctx context.Context, id string) (uint32, time.Time, error) {
	return 0, time.Time{}, nil
}

func (c *mockClient) Start(ctx context.Context, id, checkpointDir string, withStdin bool, attachStdio libcontainerd.StdioCallback) (pid int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.containers[id]; !ok {
		return 0, errors.New("not found")
	}

	if c.errorOnStart[id] {
		return 0, errors.New("some startup error")
	}
	c.containers[id] = true
	return 1, nil
}

func (c *mockClient) SignalProcess(ctx context.Context, containerID, processID string, signal int) error {
	return nil
}

func (c *mockClient) simulateStartError(sim bool, id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if sim {
		c.errorOnStart[id] = sim
		return
	}
	delete(c.errorOnStart, id)
}

func (c *mockClient) HandleExitEvent(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.containers, id)
	return nil
}
