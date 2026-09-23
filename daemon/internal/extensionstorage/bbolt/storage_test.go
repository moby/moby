package bbolt

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/extensions"
	daemonconfigv0 "github.com/moby/moby/v2/extpoints/daemonconfig/v0"
	storagekv "github.com/moby/moby/v2/extpoints/storage/kv/v0"
	storagepb "github.com/moby/moby/v2/extpoints/storage/kv/v0/protogen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

type testResolver map[extensions.PointID]any

func (r testResolver) Provider(point extensions.PointID, _ extensions.ExtensionID) (any, error) {
	p, ok := r[point]
	if !ok {
		return nil, errors.New("no provider")
	}
	return p, nil
}

func (r testResolver) Providers(point extensions.PointID) []extensions.ResolvedProvider {
	if p, ok := r[point]; ok {
		return []extensions.ResolvedProvider{{Impl: p}}
	}
	return nil
}

type testConfig struct{ root string }

func (c testConfig) Get(context.Context, *daemonconfigv0.GetRequest) (*daemonconfigv0.GetResponse, error) {
	return &daemonconfigv0.GetResponse{RootDir: c.root}, nil
}

func newTestStorage(t *testing.T, root string) storagekv.Storage {
	t.Helper()
	d := Extension.Declaration()
	assert.NilError(t, d.Init(t.Context(), nil, testResolver{daemonconfigv0.Point.ID(): testConfig{root: root}}))
	t.Cleanup(func() { assert.NilError(t, d.Shutdown(context.Background())) })
	return d.Providers[0].Impl.(storagekv.Storage)
}

func testKV(t *testing.T, provider storagekv.Storage, namespace string) *storagekv.KV {
	t.Helper()
	kv, err := storagekv.GetKV(testResolver{storagekv.Point.ID(): provider}, namespace)
	assert.NilError(t, err)
	return kv
}

func rpcStorage(t *testing.T, provider storagekv.Storage) storagekv.Storage {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	server := grpc.NewServer()
	storagepb.ServerPoint.Register(server, provider)
	t.Cleanup(server.Stop)
	go func() { _ = server.Serve(listener) }()
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NilError(t, err)
	t.Cleanup(func() { assert.NilError(t, conn.Close()) })
	return storagepb.NewClient(conn)
}

func TestKVContract(t *testing.T) {
	t.Parallel()
	for _, transport := range []string{"in-process", "RPC"} {
		t.Run(transport, func(t *testing.T) {
			t.Parallel()
			provider := newTestStorage(t, t.TempDir())
			if transport == "RPC" {
				provider = rpcStorage(t, provider)
			}
			kv := testKV(t, provider, "org.my.extension")
			other := testKV(t, provider, "org.other.extension")
			shared := testKV(t, provider, "org.my.extension")
			ctx := t.Context()

			_, err := kv.Get(ctx, "jobs/one")
			assert.Check(t, cerrdefs.IsNotFound(err), "%v", err)
			assert.Check(t, cerrdefs.IsNotFound(kv.Update(ctx, "jobs/one", []byte("missing"))))
			assert.NilError(t, kv.Delete(ctx, "jobs/one"))
			assert.NilError(t, kv.Create(ctx, "jobs/one", []byte("original")))
			err = kv.Create(ctx, "jobs/one", []byte("duplicate"))
			assert.Check(t, cerrdefs.IsConflict(err), "%v", err)
			value, err := shared.Get(ctx, "jobs/one")
			assert.NilError(t, err)
			assert.Equal(t, string(value), "original")
			value[0] = 'X'
			value, err = kv.Get(ctx, "jobs/one")
			assert.NilError(t, err)
			assert.Equal(t, string(value), "original")

			_, err = other.Get(ctx, "jobs/one")
			assert.Check(t, cerrdefs.IsNotFound(err), "%v", err)
			assert.NilError(t, other.Create(ctx, "jobs/one", []byte("other")))
			assert.NilError(t, kv.Update(ctx, "jobs/one", []byte("updated")))
			value, err = kv.Get(ctx, "jobs/one")
			assert.NilError(t, err)
			assert.Equal(t, string(value), "updated")

			// Logical keys must never be interpreted as host filesystem paths.
			assert.NilError(t, kv.Create(ctx, "../../outside", nil))
			value, err = kv.Get(ctx, "../../outside")
			assert.NilError(t, err)
			assert.Equal(t, len(value), 0)
			assert.Check(t, cerrdefs.IsConflict(kv.Create(ctx, "../../outside", nil)))
			assert.NilError(t, kv.Update(ctx, "../../outside", []byte("nonempty")))

			assert.NilError(t, kv.Create(ctx, "runs/one/a", []byte("run")))
			assert.NilError(t, kv.Delete(ctx, "jobs/one"))
			assert.NilError(t, kv.Delete(ctx, "jobs/one"))
			_, err = kv.Get(ctx, "jobs/one")
			assert.Check(t, cerrdefs.IsNotFound(err), "%v", err)
			value, err = kv.Get(ctx, "runs/one/a")
			assert.NilError(t, err)
			assert.Equal(t, string(value), "run")
			value, err = other.Get(ctx, "jobs/one")
			assert.NilError(t, err)
			assert.Equal(t, string(value), "other")

			assert.Check(t, cerrdefs.IsInvalidArgument(kv.Create(ctx, "", nil)))
			assert.Check(t, cerrdefs.IsInvalidArgument(testKV(t, provider, "../invalid").Create(ctx, "key", nil)))
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			err = kv.Create(cancelled, "cancelled", nil)
			assert.ErrorIs(t, err, context.Canceled)
			_, err = kv.Get(ctx, "cancelled")
			assert.Check(t, cerrdefs.IsNotFound(err), "%v", err)

			page, err := kv.List(ctx, storagekv.ListOptions{Prefix: "runs/"})
			assert.NilError(t, err)
			assert.DeepEqual(t, page.Keys, []string{"runs/one/a"})
			assert.Equal(t, page.NextCursor, "")
		})
	}
}

func TestStoragePersistence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	first := newTestStorage(t, root)
	kv := testKV(t, first, "jobs")
	assert.NilError(t, kv.Create(t.Context(), "job", []byte("original")))
	assert.NilError(t, kv.Update(t.Context(), "job", []byte("updated")))
	assert.NilError(t, kv.Create(t.Context(), "empty", nil))
	assert.NilError(t, kv.Create(t.Context(), "deleted", []byte("gone")))
	assert.NilError(t, kv.Delete(t.Context(), "deleted"))
	assert.NilError(t, first.(*storage).shutdown(t.Context()))

	second := newTestStorage(t, root)
	kv = testKV(t, second, "jobs")
	value, err := kv.Get(t.Context(), "job")
	assert.NilError(t, err)
	assert.Equal(t, string(value), "updated")
	value, err = kv.Get(t.Context(), "empty")
	assert.NilError(t, err)
	assert.Equal(t, len(value), 0)
	_, err = kv.Get(t.Context(), "deleted")
	assert.Check(t, cerrdefs.IsNotFound(err), "%v", err)
	_, err = first.Get(t.Context(), &storagekv.RecordRequest{Namespace: "jobs", Key: "job"})
	assert.Check(t, cerrdefs.IsUnavailable(err), "%v", err)
}

func TestStorageListing(t *testing.T) {
	t.Parallel()
	provider := newTestStorage(t, t.TempDir())
	kv := testKV(t, provider, "jobs")
	ctx := t.Context()
	page, err := kv.List(ctx, storagekv.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(page.Keys), 0)
	for _, key := range []string{"runs/b", "jobs/c", "runs/a", "jobs/a", "jobs/b"} {
		assert.NilError(t, kv.Create(ctx, key, nil))
	}
	page, err = kv.List(ctx, storagekv.ListOptions{Prefix: "jobs/", Limit: 2})
	assert.NilError(t, err)
	assert.DeepEqual(t, page.Keys, []string{"jobs/a", "jobs/b"})
	assert.Assert(t, page.NextCursor != "")
	cursor := page.NextCursor
	// Continuation does not depend on the previous key still existing.
	assert.NilError(t, kv.Delete(ctx, "jobs/b"))
	page, err = kv.List(ctx, storagekv.ListOptions{Prefix: "jobs/", Limit: 1, Cursor: cursor})
	assert.NilError(t, err)
	assert.DeepEqual(t, page.Keys, []string{"jobs/c"})
	assert.Equal(t, page.NextCursor, "")
	_, err = kv.List(ctx, storagekv.ListOptions{Prefix: "runs/", Cursor: cursor})
	assert.Check(t, cerrdefs.IsInvalidArgument(err), "%v", err)
	_, err = testKV(t, provider, "other").List(ctx, storagekv.ListOptions{Prefix: "jobs/", Cursor: cursor})
	assert.Check(t, cerrdefs.IsInvalidArgument(err), "%v", err)
	_, err = kv.List(ctx, storagekv.ListOptions{Cursor: "invalid!"})
	assert.Check(t, cerrdefs.IsInvalidArgument(err), "%v", err)
}

func TestStorageConcurrentCreate(t *testing.T) {
	t.Parallel()
	kv := testKV(t, newTestStorage(t, t.TempDir()), "jobs")
	const writers = 16
	results := make(chan error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() { results <- kv.Create(t.Context(), "same", fmt.Appendf(nil, "%d", i)) })
	}
	wg.Wait()
	close(results)
	var successes int
	for err := range results {
		if err == nil {
			successes++
		} else {
			assert.Check(t, cerrdefs.IsConflict(err), "%v", err)
		}
	}
	assert.Equal(t, successes, 1)
}

func TestStorageValidationBeforeIO(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	provider := newTestStorage(t, root)
	ctx := t.Context()
	for _, namespace := range []string{"", ".", "../escape", "/absolute", "a/b", "a\\b", "a\x00b", strings.Repeat("a", storagekv.MaxNamespaceSize+1)} {
		err := provider.Create(ctx, &storagekv.WriteRequest{Namespace: namespace, Key: "key"})
		assert.Check(t, cerrdefs.IsInvalidArgument(err), "namespace %q: %v", namespace, err)
	}
	for _, key := range []string{"", "a\x00b", "\xff", strings.Repeat("k", storagekv.MaxKeySize+1)} {
		err := provider.Create(ctx, &storagekv.WriteRequest{Namespace: "jobs", Key: key})
		assert.Check(t, cerrdefs.IsInvalidArgument(err), "key %q: %v", key, err)
	}
	assert.Check(t, cerrdefs.IsInvalidArgument(provider.Create(ctx, nil)))
	assert.Check(t, cerrdefs.IsInvalidArgument(provider.Update(ctx, nil)))
	assert.Check(t, cerrdefs.IsInvalidArgument(provider.Delete(ctx, nil)))
	_, err := provider.Get(ctx, nil)
	assert.Check(t, cerrdefs.IsInvalidArgument(err))
	_, err = provider.List(ctx, nil)
	assert.Check(t, cerrdefs.IsInvalidArgument(err))
	err = provider.Create(ctx, &storagekv.WriteRequest{Namespace: "jobs", Key: "large", Value: make([]byte, storagekv.MaxValueSize+1)})
	assert.Check(t, cerrdefs.IsInvalidArgument(err))
	_, err = provider.List(ctx, &storagekv.ListRequest{Namespace: "jobs", Limit: storagekv.MaxPageSize + 1})
	assert.Check(t, cerrdefs.IsInvalidArgument(err))
	_, err = provider.List(ctx, &storagekv.ListRequest{Namespace: "jobs", Prefix: "\xff"})
	assert.Check(t, cerrdefs.IsInvalidArgument(err))
	_, err = os.Stat(filepath.Join(root, "extensions"))
	assert.Check(t, is.ErrorIs(err, os.ErrNotExist))
}

func TestStorageLimits(t *testing.T) {
	t.Parallel()
	kv := testKV(t, rpcStorage(t, newTestStorage(t, t.TempDir())), strings.Repeat("n", storagekv.MaxNamespaceSize))
	key := strings.Repeat("k", storagekv.MaxKeySize)
	value := make([]byte, storagekv.MaxValueSize)
	assert.NilError(t, kv.Create(t.Context(), key, value))
	got, err := kv.Get(t.Context(), key)
	assert.NilError(t, err)
	assert.DeepEqual(t, got, value)
	for i := range storagekv.DefaultPageSize {
		assert.NilError(t, kv.Create(t.Context(), fmt.Sprintf("a%03d", i), nil))
	}
	page, err := kv.List(t.Context(), storagekv.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, len(page.Keys), storagekv.DefaultPageSize)
	assert.Assert(t, page.NextCursor != "")
	page, err = kv.List(t.Context(), storagekv.ListOptions{Cursor: page.NextCursor})
	assert.NilError(t, err)
	assert.DeepEqual(t, page.Keys, []string{key})
	assert.Equal(t, page.NextCursor, "")
}

func TestStorageFilesystemProtection(t *testing.T) {
	t.Parallel()
	skip.If(t, runtime.GOOS == "windows", "symlink creation requires privileges on Windows")
	for _, target := range []string{"directory", "database"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			outside := t.TempDir()
			file := filepath.Join(outside, "sentinel")
			if target == "directory" {
				assert.NilError(t, os.Symlink(outside, filepath.Join(root, "extensions")))
			} else {
				assert.NilError(t, os.WriteFile(file, []byte("untouched"), 0o644))
				// WriteFile applies the process umask; this check needs an exact mode.
				assert.NilError(t, os.Chmod(file, 0o644))
				assert.NilError(t, os.Mkdir(filepath.Join(root, "extensions"), 0o700))
				assert.NilError(t, os.Symlink(file, filepath.Join(root, "extensions", "records.db")))
			}
			kv := testKV(t, newTestStorage(t, root), "jobs")
			err := kv.Create(t.Context(), "job", nil)
			assert.ErrorContains(t, err, "symlinks are not allowed")
			if target == "directory" {
				_, err := os.Lstat(filepath.Join(outside, "records.db"))
				assert.Check(t, is.ErrorIs(err, os.ErrNotExist))
				return
			}
			data, err := os.ReadFile(file)
			assert.NilError(t, err)
			assert.Equal(t, string(data), "untouched")
			info, err := os.Stat(file)
			assert.NilError(t, err)
			assert.Equal(t, info.Mode().Perm(), os.FileMode(0o644))
		})
	}
}

func TestStoragePrivatePermissions(t *testing.T) {
	t.Parallel()
	skip.If(t, runtime.GOOS == "windows", "Unix permissions")
	root := t.TempDir()
	first := newTestStorage(t, root)
	assert.NilError(t, testKV(t, first, "jobs").Create(t.Context(), "key", nil))
	assert.NilError(t, first.(*storage).shutdown(t.Context()))
	dir := filepath.Join(root, "extensions")
	db := filepath.Join(dir, "records.db")
	assert.NilError(t, os.Chmod(dir, 0o755))
	assert.NilError(t, os.Chmod(db, 0o644))
	second := newTestStorage(t, root)
	_, err := testKV(t, second, "jobs").Get(t.Context(), "key")
	assert.NilError(t, err)
	for path, mode := range map[string]os.FileMode{dir: 0o700, db: 0o600} {
		info, err := os.Stat(path)
		assert.NilError(t, err)
		assert.Equal(t, info.Mode().Perm(), mode)
	}
}

func TestStorageDoesNotReplaceCorruptDatabase(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "extensions")
	assert.NilError(t, os.Mkdir(dir, 0o700))
	path := filepath.Join(dir, "records.db")
	assert.NilError(t, os.WriteFile(path, []byte("corrupt database"), 0o600))
	kv := testKV(t, newTestStorage(t, root), "jobs")
	_, err := kv.Get(t.Context(), "key")
	assert.Assert(t, err != nil)
	assert.Check(t, !cerrdefs.IsNotFound(err))
	data, err := os.ReadFile(path)
	assert.NilError(t, err)
	assert.Equal(t, string(data), "corrupt database")
}

func TestStorageInitialization(t *testing.T) {
	t.Parallel()
	for _, root := range []string{"", "relative"} {
		d := Extension.Declaration()
		err := d.Init(t.Context(), nil, testResolver{daemonconfigv0.Point.ID(): testConfig{root: root}})
		assert.Assert(t, err != nil)
		assert.NilError(t, d.Shutdown(t.Context()))
	}
	d := Extension.Declaration()
	assert.Assert(t, d.Init(t.Context(), nil, testResolver{}) != nil)
	assert.NilError(t, d.Shutdown(t.Context()))
	_, err := storagekv.GetKV(testResolver{}, "jobs")
	assert.Assert(t, err != nil)
}
