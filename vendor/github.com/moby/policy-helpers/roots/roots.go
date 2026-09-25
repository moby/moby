package roots

import (
	"context"
	"embed"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/pkg/errors"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/fetcher"
)

type SigstoreRootsConfig struct {
	CachePath      string
	UpdateInterval time.Duration
	RequireOnline  bool
}

type TrustProvider struct {
	mu      sync.RWMutex
	config  SigstoreRootsConfig
	client  *tuf.Client
	fetcher *airgappedFetcher

	status Status
}

type Status struct {
	Error       error      `json:"error,omitempty"`
	LastUpdated *time.Time `json:"lastUpdated,omitempty"`
}

const (
	trustedRootFilename = "trusted_root.json"
)

func NewTrustProvider(cfg SigstoreRootsConfig) (*TrustProvider, error) {
	if cfg.CachePath == "" {
		return nil, errors.Errorf("cache path must be provided for trust provider")
	}
	def := tuf.DefaultOptions()
	cacheDir := filepath.Join(cfg.CachePath, tuf.URLToPath(def.RepositoryBaseURL))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, errors.Wrap(err, "creating cache directory for trust provider")
	}

	tp := &TrustProvider{
		config: cfg,
	}

	unlock, err := tp.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()

	root, err := os.OpenRoot(cacheDir)
	if err != nil {
		return nil, errors.Wrap(err, "opening cache directory for trust provider")
	}
	defer root.Close()
	if _, err := root.Lstat("root.json"); err != nil {
		if !os.IsNotExist(err) {
			return nil, errors.Wrap(err, "statting root.json in cache directory for trust provider")
		}
		if err := copyEmbeddedRoot(EmbeddedTUF, root); err != nil {
			return nil, errors.Wrap(err, "initializing cache directory for trust provider with embedded root")
		}
	}

	agf := &airgappedFetcher{
		baseURL:       def.RepositoryBaseURL,
		cacheDir:      cacheDir,
		onlineFetcher: fetcher.NewDefaultFetcher(),
		isOnline:      true,
	}
	tp.fetcher = agf

	tufOpts, err := tp.tufClientOpts()
	if err != nil {
		return nil, errors.Wrap(err, "creating TUF client options for trust provider")
	}

	c, err := tuf.New(tufOpts)
	if err != nil {
		// try again with airgapped fetcher
		// this can still fail if the last root or timestamps file has expired

		agf.isOnline = false
		tufOpts, err := tp.tufClientOpts()
		if err != nil {
			return nil, errors.Wrap(err, "creating TUF client options for trust provider with airgapped fetcher")
		}
		c, err = tuf.New(tufOpts)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	tp.client = c
	agf.isOnline = true

	go tp.update()

	if cfg.UpdateInterval > 0 {
		go func() {
			ticker := time.NewTicker(cfg.UpdateInterval)
			defer ticker.Stop()
			// TODO: stop condition
			for range ticker.C {
				tp.update()
			}
		}()
	}

	return tp, nil
}

func (tp *TrustProvider) tufClientOpts() (*tuf.Options, error) {
	def := tuf.DefaultOptions()
	cacheDir := filepath.Join(tp.config.CachePath, tuf.URLToPath(def.RepositoryBaseURL))
	root, err := os.OpenRoot(cacheDir)
	if err != nil {
		return nil, errors.Wrap(err, "opening cache directory for trust provider")
	}
	defer root.Close()

	dt, err := EmbeddedTUF.ReadFile("tuf-root/root.json")
	if err != nil {
		return nil, err
	}
	def.Root = dt
	def.CachePath = tp.config.CachePath
	def.ForceCache = !tp.config.RequireOnline
	def.Fetcher = tp.fetcher
	return def, nil
}

func (tp *TrustProvider) update() (err error) {
	defer func() {
		if err != nil {
			tp.mu.Lock()
			tp.status.Error = err
			tp.mu.Unlock()
		}
	}()

	unlock, err := tp.lock()
	if err != nil {
		return err
	}
	defer unlock()
	tufOpts, err := tp.tufClientOpts()
	if err != nil {
		return errors.Wrap(err, "creating TUF client options for trust provider")
	}
	c, err := tuf.New(tufOpts)
	if err != nil {
		return errors.WithStack(err)
	}
	err = c.Refresh()
	if err != nil {
		return err
	}
	tp.mu.Lock()
	defer tp.mu.Unlock()
	now := time.Now().UTC()
	tp.status = Status{LastUpdated: &now}
	tp.client = c
	return nil
}

func (tp *TrustProvider) wait(ctx context.Context) (*tuf.Client, error) {
	first := true
	errCh := make(chan error, 1)
	for {
		tp.mu.RLock()
		status := tp.status
		client := tp.client
		tp.mu.RUnlock()
		if status.LastUpdated != nil && status.Error == nil {
			return client, nil
		}
		// try update if we are in error from some old reason that might be resolved now
		if status.Error != nil && first {
			go func() {
				errCh <- tp.update()
			}()
			first = false
		}
		select {
		case err := <-errCh:
			if err == nil {
				continue
			}
			return nil, err
		case <-ctx.Done():
			return nil, context.Cause(ctx)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (tp *TrustProvider) lock() (func() error, error) {
	lockPath := path.Join(tp.config.CachePath, ".lock")
	fileLock := flock.New(lockPath)
	if err := fileLock.Lock(); err != nil {
		return nil, errors.Wrap(err, "acquiring lock on trust provider cache")
	}
	return fileLock.Unlock, nil
}

func (tp *TrustProvider) TrustedRoot(ctx context.Context) (*root.TrustedRoot, Status, error) {
	ctx, cnclFn := context.WithCancelCause(ctx)
	ctx, _ = context.WithTimeoutCause(ctx, time.Second*5, errors.WithStack(context.DeadlineExceeded)) //nolint:govet
	defer cnclFn(errors.WithStack(context.Canceled))

	var st Status
	client, err := tp.wait(ctx)
	if err != nil { // return indication of last refresh error? TODO(@tonistiigi) does this make GetTarget fail as well and separate instance of client is needed for optional refresh?
		st.Error = err
		tp.mu.RLock()
		client = tp.client
		tp.mu.RUnlock()
	}

	jsonBytes, err := client.GetTarget(trustedRootFilename)
	if err != nil {
		return nil, st, err
	}
	tr, err := root.NewTrustedRootFromJSON(jsonBytes)
	return tr, st, err
}

type airgappedFetcher struct {
	baseURL       string
	cacheDir      string
	onlineFetcher fetcher.Fetcher
	isOnline      bool
}

func (f *airgappedFetcher) DownloadFile(urlPath string, maxLength int64, dur time.Duration) ([]byte, error) {
	if f.isOnline {
		dt, err := f.onlineFetcher.DownloadFile(urlPath, maxLength, dur)
		if err != nil {
			return nil, err
		}
		// save root chain to cache so that it can be reverified while offline
		u, err := url.Parse(urlPath)
		if err != nil {
			return nil, errors.Wrap(err, "parsing URL in trust provider fetcher")
		}
		cache, err := os.OpenRoot(f.cacheDir)
		if err != nil {
			return nil, errors.Wrap(err, "opening cache directory for trust provider")
		}
		defer cache.Close()
		if strings.HasSuffix(u.Path, ".root.json") {
			base := path.Base(u.Path)
			if err := cache.MkdirAll("roots", 0o755); err != nil {
				return nil, errors.Wrap(err, "creating roots directory in trust provider cache")
			}
			if err := cache.WriteFile(path.Join("roots", base), dt, 0o644); err != nil {
				return nil, errors.Wrap(err, "caching root file in trust provider cache")
			}
		}
		return dt, nil
	}
	const timestampFilename = "timestamp.json"
	if urlPath == f.baseURL+"/"+timestampFilename {
		cache, err := os.OpenRoot(f.cacheDir)
		if err != nil {
			return nil, errors.Wrap(err, "opening cache directory for trust provider")
		}
		defer cache.Close()
		if dt, err := cache.ReadFile(timestampFilename); err == nil {
			return dt, nil
		}
	}
	if strings.HasSuffix(urlPath, ".root.json") {
		u, err := url.Parse(urlPath)
		if err == nil {
			base := path.Base(u.Path)
			if urlPath == f.baseURL+"/"+base && strings.HasSuffix(base, ".root.json") {
				cache, err := os.OpenRoot(f.cacheDir)
				if err != nil {
					return nil, errors.Wrap(err, "opening cache directory for trust provider")
				}
				defer cache.Close()
				dt, err := cache.ReadFile("roots/" + base)
				if err == nil {
					return dt, nil
				}
			}
		}
	}
	return nil, &metadata.ErrDownloadHTTP{
		StatusCode: 404,
	}
}

func copyEmbeddedRoot(src embed.FS, dest *os.Root) error {
	subFS, err := fs.Sub(src, "tuf-root")
	if err != nil {
		return errors.WithStack(err)
	}
	return fs.WalkDir(subFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return dest.MkdirAll(p, 0o755)
		}
		in, err := subFS.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()

		if err := dest.MkdirAll(path.Dir(p), 0o755); err != nil {
			return err
		}
		out, err := dest.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}
