package dockerfile

import (
	"os"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/remotecontext"
	"github.com/docker/docker/client/session/fssession"
	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
	"golang.org/x/net/context"
	"golang.org/x/sync/singleflight"
)

// TODO: move to separate package

// FSCacheBackend is a backing implementation for FSCache
type FSCacheBackend interface {
	Get(id string) (string, error)
	Remove(id string) error
}

// FSCache allows syncing remote resources to cached snapshots
type FSCache struct {
	opt        FSCacheOpt
	contexts   map[string]*remoteContext
	transports map[string]Transport
	mu         sync.Mutex
	g          singleflight.Group
}

// FSCacheOpt defines options for initializing FSCache
type FSCacheOpt struct {
	Backend  FSCacheBackend
	Root     string // for storing local metadata
	MaxUsage int64
}

// NewFSCache returns new FSCache object
func NewFSCache(opt FSCacheOpt) (*FSCache, error) {
	return &FSCache{
		opt:        opt,
		contexts:   make(map[string]*remoteContext),
		transports: make(map[string]Transport),
	}, nil
}

// Transport defines a method for syncing remote data to FSCache
type Transport interface {
	Copy(ctx context.Context, id RemoteIdentifier, dest string, cs fssession.CacheUpdater) error
}

// RemoteIdentifier identifies a transfer request
type RemoteIdentifier interface {
	Key() string
	SharedKey() string
	Transport() string
}

type remoteContext struct {
	backend    FSCacheBackend
	references map[*wrappedContext]struct{}
	sharedKey  string
	ctx        builder.Source
	backendID  string
	mu         sync.Mutex
	path       string
	tarsum     *fsutil.Tarsum
}

func newRemoteContext(sharedKey string, b FSCacheBackend) (*remoteContext, error) {
	backendID := stringid.GenerateRandomID()
	return &remoteContext{
		backendID:  backendID,
		backend:    b,
		sharedKey:  sharedKey,
		references: make(map[*wrappedContext]struct{}),
	}, nil
}

func (rc *remoteContext) syncFrom(ctx context.Context, transport Transport, id RemoteIdentifier) error {
	rc.mu.Lock()
	if rc.path == "" {
		p, err := rc.backend.Get(rc.backendID)
		if err != nil {
			rc.mu.Unlock()
			return errors.Wrapf(err, "failed to get backend storage path for %s", rc.backendID)
		}
		rc.path = p
		rc.tarsum = fsutil.NewTarsum(p)
	}

	path := rc.path
	rc.mu.Unlock()

	var dc *detectChanges
	if rc.tarsum != nil {
		dc = &detectChanges{f: rc.tarsum.HandleChange}
	}

	if err := transport.Copy(ctx, id, path, dc); err != nil {
		return errors.Wrapf(err, "failed to copy to %s", path)
	}
	rc.mu.Lock()
	if dc != nil && dc.supported {
		rc.ctx = rc.tarsum
	} else {
		rc.tarsum = nil
		ctx, err := remotecontext.NewLazyContext(path)
		if err != nil {
			return errors.Wrapf(err, "failed to make lazycontext from %s", path)
		}
		rc.ctx = ctx
	}
	rc.mu.Unlock()

	return nil
}

// temporary
type detectChanges struct {
	f         fsutil.ChangeFunc
	supported bool
}

func (dc *detectChanges) HandleChange(kind fsutil.ChangeKind, path string, fi os.FileInfo, err error) error {
	if dc == nil {
		return nil
	}
	return dc.f(kind, path, fi, err)
}

func (dc *detectChanges) MarkSupported(v bool) {
	if dc == nil {
		return
	}
	dc.supported = v
}

func (rc *remoteContext) context() (builder.Source, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	wc := &wrappedContext{Source: rc.ctx}
	wc.closer = func() error {
		rc.mu.Lock()
		delete(rc.references, wc)
		rc.mu.Unlock()
		return nil
	}
	rc.references[wc] = struct{}{}
	return wc, nil
}

func (rc *remoteContext) unused() bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return len(rc.references) == 0
}

func (rc *remoteContext) cleanup() error {
	rc.mu.Lock()
	if len(rc.references) == 0 {
		rc.backend.Remove(rc.backendID)
		rc.path = ""
	}
	rc.mu.Unlock()
	return nil
}

func (rc *remoteContext) clone() *remoteContext {
	return rc
}

// RegisterTransport registers a new transport method
func (fsc *FSCache) RegisterTransport(id string, transport Transport) error {
	fsc.mu.Lock()
	defer fsc.mu.Unlock()
	if _, ok := fsc.transports[id]; ok {
		return errors.Errorf("transport %v already exists", id)
	}
	fsc.transports[id] = transport
	return nil
}

// SyncFrom returns a source based on a remote identifier
func (fsc *FSCache) SyncFrom(ctx context.Context, id RemoteIdentifier) (builder.Source, error) { // cacheOpt
	trasportID := id.Transport()
	fsc.mu.Lock()
	tr, ok := fsc.transports[id.Transport()]
	if !ok {
		fsc.mu.Unlock()
		return nil, errors.Errorf("invalid transport %s", trasportID)
	}

	logrus.Debugf("SyncFrom %s %s", id.Key(), id.SharedKey())
	fsc.mu.Unlock()
	rc, err, _ := fsc.g.Do(id.Key(), func() (interface{}, error) {
		var rc *remoteContext
		fsc.mu.Lock()
		rc, ok := fsc.contexts[id.Key()]
		if ok {
			fsc.mu.Unlock()
			return rc, nil
		}

		// check for unused shared cache
		sharedKey := id.SharedKey()
		if sharedKey != "" {
			for id, rctx := range fsc.contexts {
				if rctx.unused() && rctx.sharedKey == sharedKey {
					rc = rctx.clone()
					delete(fsc.contexts, id)
				}
			}
		}
		fsc.mu.Unlock()

		if rc == nil {
			var err error
			rc, err = newRemoteContext(sharedKey, fsc.opt.Backend)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create remote context")
			}
		}

		if err := rc.syncFrom(ctx, tr, id); err != nil {
			rc.cleanup()
			return nil, err
		}

		fsc.mu.Lock()
		fsc.contexts[id.Key()] = rc
		fsc.mu.Unlock()
		return rc, nil
	})

	if err != nil {
		return nil, err
	}
	remoteContext := rc.(*remoteContext)

	r, err := remoteContext.context()
	if err == nil {
		logrus.Debugf("remote: %s", r.Root())
	}
	return r, err
}

// DiskUsage reports how much data is allocated by the cache
func (fsc *FSCache) DiskUsage() (int64, int64, error) {
	return -1, -1, errors.New("not implemented")
}

// Purge allows manually cleaning up the cache
func (fsc *FSCache) Purge() error {
	return errors.New("not implemented")
}

// GC runs a garbage collector on FSCache
func (fsc *FSCache) GC() error {
	return errors.New("not implemented")
}

type wrappedContext struct {
	builder.Source
	closer func() error
}

func (wc *wrappedContext) Close() error {
	if err := wc.Source.Close(); err != nil {
		return err
	}
	return wc.closer()
}
