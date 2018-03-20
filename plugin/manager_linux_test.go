package plugin // import "github.com/docker/docker/plugin"

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/plugin/v2"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func TestManagerWithPluginMounts(t *testing.T) {
	root, err := ioutil.TempDir("", "test-store-with-plugin-mounts")
	if err != nil {
		t.Fatal(err)
	}
	defer system.EnsureRemoveAll(root)

	s := NewStore()
	managerRoot := filepath.Join(root, "manager")
	p1 := newTestPlugin(t, "test1", "testcap", managerRoot)

	p2 := newTestPlugin(t, "test2", "testcap", managerRoot)
	p2.PluginObj.Enabled = true

	m, err := NewManager(
		ManagerConfig{
			Store:          s,
			Root:           managerRoot,
			ExecRoot:       filepath.Join(root, "exec"),
			CreateExecutor: func(*Manager) (Executor, error) { return nil, nil },
			LogPluginEvent: func(_, _, _ string) {},
		})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Add(p1); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(p2); err != nil {
		t.Fatal(err)
	}

	// Create a mount to simulate a plugin that has created it's own mounts
	p2Mount := filepath.Join(p2.Rootfs, "testmount")
	if err := os.MkdirAll(p2Mount, 0755); err != nil {
		t.Fatal(err)
	}
	if err := mount.Mount("tmpfs", p2Mount, "tmpfs", ""); err != nil {
		t.Fatal(err)
	}

	if err := m.Remove(p1.Name(), &types.PluginRmConfig{ForceRemove: true}); err != nil {
		t.Fatal(err)
	}
	if mounted, err := mount.Mounted(p2Mount); !mounted || err != nil {
		t.Fatalf("expected %s to be mounted, err: %v", p2Mount, err)
	}
}

func newTestPlugin(t *testing.T, name, cap, root string) *v2.Plugin {
	rootfs := filepath.Join(root, name)
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		t.Fatal(err)
	}

	p := v2.Plugin{PluginObj: types.Plugin{Name: name}}
	p.Rootfs = rootfs
	iType := types.PluginInterfaceType{Capability: cap, Prefix: "docker", Version: "1.0"}
	i := types.PluginConfigInterface{Socket: "plugins.sock", Types: []types.PluginInterfaceType{iType}}
	p.PluginObj.Config.Interface = i
	p.PluginObj.ID = name

	return &p
}

type simpleExecutor struct {
}

func (e *simpleExecutor) Create(id string, spec specs.Spec, stdout, stderr io.WriteCloser) error {
	return errors.New("Create failed")
}

func (e *simpleExecutor) Restore(id string, stdout, stderr io.WriteCloser) error {
	return nil
}

func (e *simpleExecutor) IsRunning(id string) (bool, error) {
	return false, nil
}

func (e *simpleExecutor) Signal(id string, signal int) error {
	return nil
}

func TestCreateFailed(t *testing.T) {
	root, err := ioutil.TempDir("", "test-create-failed")
	if err != nil {
		t.Fatal(err)
	}
	defer system.EnsureRemoveAll(root)

	s := NewStore()
	managerRoot := filepath.Join(root, "manager")
	p := newTestPlugin(t, "create", "testcreate", managerRoot)

	m, err := NewManager(
		ManagerConfig{
			Store:          s,
			Root:           managerRoot,
			ExecRoot:       filepath.Join(root, "exec"),
			CreateExecutor: func(*Manager) (Executor, error) { return &simpleExecutor{}, nil },
			LogPluginEvent: func(_, _, _ string) {},
		})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Add(p); err != nil {
		t.Fatal(err)
	}

	if err := m.enable(p, &controller{}, false); err == nil {
		t.Fatalf("expected Create failed error, got %v", err)
	}

	if err := m.Remove(p.Name(), &types.PluginRmConfig{ForceRemove: true}); err != nil {
		t.Fatal(err)
	}
}
