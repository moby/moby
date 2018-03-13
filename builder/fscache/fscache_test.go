package fscache // import "github.com/docker/docker/builder/fscache"

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/moby/buildkit/session/filesync"
	"golang.org/x/net/context"
)

func TestFSCache(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "fscache")
	assert.Check(t, err)
	defer os.RemoveAll(tmpDir)

	backend := NewNaiveCacheBackend(filepath.Join(tmpDir, "backend"))

	opt := Opt{
		Root:     tmpDir,
		Backend:  backend,
		GCPolicy: GCPolicy{MaxSize: 15, MaxKeepDuration: time.Hour},
	}

	fscache, err := NewFSCache(opt)
	assert.Check(t, err)

	defer fscache.Close()

	err = fscache.RegisterTransport("test", &testTransport{})
	assert.Check(t, err)

	src1, err := fscache.SyncFrom(context.TODO(), &testIdentifier{"foo", "data", "bar"})
	assert.Check(t, err)

	dt, err := ioutil.ReadFile(filepath.Join(src1.Root().Path(), "foo"))
	assert.Check(t, err)
	assert.Check(t, is.Equal(string(dt), "data"))

	// same id doesn't recalculate anything
	src2, err := fscache.SyncFrom(context.TODO(), &testIdentifier{"foo", "data2", "bar"})
	assert.Check(t, err)
	assert.Check(t, is.Equal(src1.Root().Path(), src2.Root().Path()))

	dt, err = ioutil.ReadFile(filepath.Join(src1.Root().Path(), "foo"))
	assert.Check(t, err)
	assert.Check(t, is.Equal(string(dt), "data"))
	assert.Check(t, src2.Close())

	src3, err := fscache.SyncFrom(context.TODO(), &testIdentifier{"foo2", "data2", "bar"})
	assert.Check(t, err)
	assert.Check(t, src1.Root().Path() != src3.Root().Path())

	dt, err = ioutil.ReadFile(filepath.Join(src3.Root().Path(), "foo2"))
	assert.Check(t, err)
	assert.Check(t, is.Equal(string(dt), "data2"))

	s, err := fscache.DiskUsage()
	assert.Check(t, err)
	assert.Check(t, is.Equal(s, int64(0)))

	assert.Check(t, src3.Close())

	s, err = fscache.DiskUsage()
	assert.Check(t, err)
	assert.Check(t, is.Equal(s, int64(5)))

	// new upload with the same shared key shoutl overwrite
	src4, err := fscache.SyncFrom(context.TODO(), &testIdentifier{"foo3", "data3", "bar"})
	assert.Check(t, err)
	assert.Check(t, src1.Root().Path() != src3.Root().Path())

	dt, err = ioutil.ReadFile(filepath.Join(src3.Root().Path(), "foo3"))
	assert.Check(t, err)
	assert.Check(t, is.Equal(string(dt), "data3"))
	assert.Check(t, is.Equal(src4.Root().Path(), src3.Root().Path()))
	assert.Check(t, src4.Close())

	s, err = fscache.DiskUsage()
	assert.Check(t, err)
	assert.Check(t, is.Equal(s, int64(10)))

	// this one goes over the GC limit
	src5, err := fscache.SyncFrom(context.TODO(), &testIdentifier{"foo4", "datadata", "baz"})
	assert.Check(t, err)
	assert.Check(t, src5.Close())

	// GC happens async
	time.Sleep(100 * time.Millisecond)

	// only last insertion after GC
	s, err = fscache.DiskUsage()
	assert.Check(t, err)
	assert.Check(t, is.Equal(s, int64(8)))

	// prune deletes everything
	released, err := fscache.Prune(context.TODO())
	assert.Check(t, err)
	assert.Check(t, is.Equal(released, uint64(8)))

	s, err = fscache.DiskUsage()
	assert.Check(t, err)
	assert.Check(t, is.Equal(s, int64(0)))
}

type testTransport struct {
}

func (t *testTransport) Copy(ctx context.Context, id RemoteIdentifier, dest string, cs filesync.CacheUpdater) error {
	testid := id.(*testIdentifier)
	return ioutil.WriteFile(filepath.Join(dest, testid.filename), []byte(testid.data), 0600)
}

type testIdentifier struct {
	filename  string
	data      string
	sharedKey string
}

func (t *testIdentifier) Key() string {
	return t.filename
}
func (t *testIdentifier) SharedKey() string {
	return t.sharedKey
}
func (t *testIdentifier) Transport() string {
	return "test"
}
