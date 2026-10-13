package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/distribution/reference"
	"github.com/moby/moby/v2/daemon/pkg/registry"
	"gotest.tools/v3/assert"
)

func TestAtomicRemoveAllNormal(t *testing.T) {
	dir := t.TempDir()

	if err := atomicRemoveAll(dir); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone: %v", err)
	}
	if _, err := os.Stat(dir + "-removing"); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone: %v", err)
	}
}

func TestAtomicRemoveAllAlreadyExists(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(dir+"-removing", 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir + "-removing")

	if err := atomicRemoveAll(dir); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone: %v", err)
	}
	if _, err := os.Stat(dir + "-removing"); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone: %v", err)
	}
}

func TestAtomicRemoveAllNotExist(t *testing.T) {
	if err := atomicRemoveAll("/not-exist"); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()

	// create the removing dir, but not the "real" one
	foo := filepath.Join(dir, "foo")
	removing := dir + "-removing"
	if err := os.MkdirAll(removing, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := atomicRemoveAll(dir); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(foo); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone: %v", err)
	}
	if _, err := os.Stat(removing); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone: %v", err)
	}
}

func TestPrivilegesFetchError(t *testing.T) {
	want := errors.New("registry lookup failed")
	pm := &Manager{config: ManagerConfig{RegistryService: failingEndpointResolver{err: want}}}
	ref, err := reference.ParseNormalizedNamed("example.com/test/plugin:latest")
	assert.NilError(t, err)

	_, err = pm.Privileges(t.Context(), ref, nil, nil)
	assert.ErrorIs(t, err, want)
}

type failingEndpointResolver struct{ err error }

func (r failingEndpointResolver) LookupPullEndpoints(string) ([]registry.APIEndpoint, error) {
	return nil, r.err
}
