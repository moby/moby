/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerd/errdefs"
	"github.com/containerd/log"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/internal/fsverity"
	"github.com/containerd/containerd/v2/pkg/filters"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, 1<<20)
		return &buffer
	},
}

// LabelStore is used to store mutable labels for digests
type LabelStore interface {
	// Get returns all the labels for the given digest
	Get(digest.Digest) (map[string]string, error)

	// Set sets all the labels for a given digest
	Set(digest.Digest, map[string]string) error

	// Update replaces the given labels for a digest,
	// a key with an empty value removes a label.
	Update(digest.Digest, map[string]string) (map[string]string, error)
}

// Store is digest-keyed store for content. All data written into the store is
// stored under a verifiable digest.
//
// Store can generally support multi-reader, single-writer ingest of data,
// including resumable ingest.
type store struct {
	root               string
	ls                 LabelStore
	integritySupported bool

	locksMu              sync.Mutex
	locks                map[string]*lock
	ensureIngestRootOnce func() error
}

// NewStore returns a local content store
func NewStore(root string) (content.Store, error) {
	return NewLabeledStore(root, nil)
}

// NewLabeledStore returns a new content store using the provided label store
//
// Note: content stores which are used underneath a metadata store may not
// require labels and should use `NewStore`. `NewLabeledStore` is primarily
// useful for tests or standalone implementations.
func NewLabeledStore(root string, ls LabelStore) (content.Store, error) {
	supported, _ := fsverity.IsSupported(root)

	s := &store{
		root:               root,
		ls:                 ls,
		integritySupported: supported,
		locks:              map[string]*lock{},
	}
	s.ensureIngestRootOnce = sync.OnceValue(s.ensureIngestRoot)
	return s, nil
}

func (s *store) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	p, err := s.blobPath(dgst)
	if err != nil {
		return content.Info{}, fmt.Errorf("calculating blob info path: %w", err)
	}

	fi, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			err = fmt.Errorf("content %v: %w", dgst, errdefs.ErrNotFound)
		}

		return content.Info{}, err
	}
	var labels map[string]string
	if s.ls != nil {
		labels, err = s.ls.Get(dgst)
		if err != nil {
			return content.Info{}, err
		}
	}
	return s.info(dgst, fi, labels), nil
}

func (s *store) info(dgst digest.Digest, fi os.FileInfo, labels map[string]string) content.Info {
	return content.Info{
		Digest:    dgst,
		Size:      fi.Size(),
		CreatedAt: fi.ModTime(),
		UpdatedAt: getATime(fi),
		Labels:    labels,
	}
}

// ReaderAt returns an io.ReaderAt for the blob.
func (s *store) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	p, err := s.blobPath(desc.Digest)
	if err != nil {
		return nil, fmt.Errorf("calculating blob path for ReaderAt: %w", err)
	}

	reader, err := OpenReader(p)
	if err != nil {
		return nil, fmt.Errorf("blob %s expected at %s: %w", desc.Digest, p, err)
	}

	return reader, nil
}

// Delete removes a blob by its digest.
//
// While this is safe to do concurrently, safe exist-removal logic must hold
// some global lock on the store.
func (s *store) Delete(ctx context.Context, dgst digest.Digest) error {
	bp, err := s.blobPath(dgst)
	if err != nil {
		return fmt.Errorf("calculating blob path for delete: %w", err)
	}

	if err := os.RemoveAll(bp); err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		return fmt.Errorf("content %v: %w", dgst, errdefs.ErrNotFound)
	}

	return nil
}

func (s *store) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	if s.ls == nil {
		return content.Info{}, fmt.Errorf("update not supported on immutable content store: %w", errdefs.ErrFailedPrecondition)
	}

	p, err := s.blobPath(info.Digest)
	if err != nil {
		return content.Info{}, fmt.Errorf("calculating blob path for update: %w", err)
	}

	fi, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			err = fmt.Errorf("content %v: %w", info.Digest, errdefs.ErrNotFound)
		}

		return content.Info{}, err
	}

	var (
		all    bool
		labels map[string]string
	)
	if len(fieldpaths) > 0 {
		for _, path := range fieldpaths {
			if strings.HasPrefix(path, "labels.") {
				if labels == nil {
					labels = map[string]string{}
				}

				key := strings.TrimPrefix(path, "labels.")
				labels[key] = info.Labels[key]
				continue
			}

			switch path {
			case "labels":
				all = true
				labels = info.Labels
			default:
				return content.Info{}, fmt.Errorf("cannot update %q field on content info %q: %w", path, info.Digest, errdefs.ErrInvalidArgument)
			}
		}
	} else {
		all = true
		labels = info.Labels
	}

	if all {
		err = s.ls.Set(info.Digest, labels)
	} else {
		labels, err = s.ls.Update(info.Digest, labels)
	}
	if err != nil {
		return content.Info{}, err
	}

	info = s.info(info.Digest, fi, labels)
	info.UpdatedAt = time.Now()

	if err := os.Chtimes(p, info.UpdatedAt, info.CreatedAt); err != nil {
		log.G(ctx).WithError(err).Warnf("could not change access time for %s", info.Digest)
	}

	return info, nil
}

func (s *store) Walk(ctx context.Context, fn content.WalkFunc, fs ...string) error {
	root := filepath.Join(s.root, "blobs")

	filter, err := filters.ParseAll(fs...)
	if err != nil {
		return err
	}

	var alg digest.Algorithm
	return filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.IsDir() && !alg.Available() {
			return nil
		}

		// TODO(stevvooe): There are few more cases with subdirs that should be
		// handled in case the layout gets corrupted. This isn't strict enough
		// and may spew bad data.

		if path == root {
			return nil
		}
		if filepath.Dir(path) == root {
			alg = digest.Algorithm(filepath.Base(path))

			if !alg.Available() {
				alg = ""
				return filepath.SkipDir
			}

			// descending into a hash directory
			return nil
		}

		dgst := digest.NewDigestFromEncoded(alg, filepath.Base(path))
		if err := dgst.Validate(); err != nil {
			// log error but don't report
			log.L.WithError(err).WithField("path", path).Error("invalid digest for blob path")
			// if we see this, it could mean some sort of corruption of the
			// store or extra paths not expected previously.
		}

		var labels map[string]string
		if s.ls != nil {
			labels, err = s.ls.Get(dgst)
			if err != nil {
				return err
			}
		}

		info := s.info(dgst, fi, labels)
		if !filter.Match(content.AdaptInfo(info)) {
			return nil
		}
		return fn(info)
	})
}

func (s *store) Status(ctx context.Context, ref string) (content.Status, error) {
	return s.status(s.ingestRoot(ref))
}

func (s *store) ListStatuses(ctx context.Context, fs ...string) ([]content.Status, error) {
	fp, err := os.Open(filepath.Join(s.root, "ingest"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer fp.Close()

	fis, err := fp.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	filter, err := filters.ParseAll(fs...)
	if err != nil {
		return nil, err
	}

	var active []content.Status
	for _, fi := range fis {
		p := filepath.Join(s.root, "ingest", fi)
		stat, err := s.status(p)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}

			// TODO(stevvooe): This is a common error if uploads are being
			// completed while making this listing. Need to consider taking a
			// lock on the whole store to coordinate this aspect.
			//
			// Another option is to cleanup downloads asynchronously and
			// coordinate this method with the cleanup process.
			//
			// For now, we just skip them, as they really don't exist.
			continue
		}

		if filter.Match(adaptStatus(stat)) {
			active = append(active, stat)
		}
	}

	return active, nil
}

// WalkStatusRefs is used to walk all status references
// Failed status reads will be logged and ignored, if
// this function is called while references are being altered,
// these error messages may be produced.
func (s *store) WalkStatusRefs(ctx context.Context, fn func(string) error) error {
	fp, err := os.Open(filepath.Join(s.root, "ingest"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer fp.Close()

	fis, err := fp.Readdirnames(-1)
	if err != nil {
		return err
	}

	for _, fi := range fis {
		rf := filepath.Join(s.root, "ingest", fi, "ref")

		ref, err := readFileString(rf)
		if err != nil {
			log.G(ctx).WithError(err).WithField("path", rf).Error("failed to read ingest ref")
			continue
		}

		if err := fn(ref); err != nil {
			return err
		}
	}

	return nil
}

// status works like stat above except uses the path to the ingest.
func (s *store) status(ingestPath string) (content.Status, error) {
	dp := filepath.Join(ingestPath, "data")
	fi, err := os.Stat(dp)
	if err != nil {
		if os.IsNotExist(err) {
			err = fmt.Errorf("%s: %w", err.Error(), errdefs.ErrNotFound)
		}
		return content.Status{}, err
	}

	ref, err := readFileString(filepath.Join(ingestPath, "ref"))
	if err != nil {
		if os.IsNotExist(err) {
			err = fmt.Errorf("%s: %w", err.Error(), errdefs.ErrNotFound)
		}
		return content.Status{}, err
	}

	startedAt, err := readFileTimestamp(filepath.Join(ingestPath, "startedat"))
	if err != nil {
		return content.Status{}, fmt.Errorf("could not read startedat: %w", err)
	}

	updatedAt, err := readFileTimestamp(filepath.Join(ingestPath, "updatedat"))
	if err != nil {
		return content.Status{}, fmt.Errorf("could not read updatedat: %w", err)
	}

	// because we don't write updatedat on every write, the mod time may
	// actually be more up to date.
	if fi.ModTime().After(updatedAt) {
		updatedAt = fi.ModTime()
	}

	return content.Status{
		Ref:       ref,
		Offset:    fi.Size(),
		Total:     s.total(ingestPath),
		UpdatedAt: updatedAt,
		StartedAt: startedAt,
	}, nil
}

func adaptStatus(status content.Status) filters.Adaptor {
	return filters.AdapterFunc(func(fieldpath []string) (string, bool) {
		if len(fieldpath) == 0 {
			return "", false
		}
		switch fieldpath[0] {
		case "ref":
			return status.Ref, true
		}

		return "", false
	})
}

// total attempts to resolve the total expected size for the write.
func (s *store) total(ingestPath string) int64 {
	totalS, err := readFileString(filepath.Join(ingestPath, "total"))
	if err != nil {
		return 0
	}

	total, err := strconv.ParseInt(totalS, 10, 64)
	if err != nil {
		// represents a corrupted file, should probably remove.
		return 0
	}

	return total
}

// Writer begins or resumes the active writer identified by ref. If the writer
// is already in use, an error is returned. Only one writer may be in use per
// ref at a time.
//
// The argument `ref` is used to uniquely identify a long-lived writer transaction.
func (s *store) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	var wOpts content.WriterOpts
	for _, opt := range opts {
		if err := opt(&wOpts); err != nil {
			return nil, err
		}
	}
	// TODO(AkihiroSuda): we could create a random string or one calculated based on the context
	// https://github.com/containerd/containerd/issues/2129#issuecomment-380255019
	if wOpts.Ref == "" {
		return nil, fmt.Errorf("ref must not be empty: %w", errdefs.ErrInvalidArgument)
	}

	if err := s.tryLock(wOpts.Ref); err != nil {
		return nil, err
	}

	w, err := s.writer(ctx, wOpts.Ref, wOpts.Desc.Size, wOpts.Desc.Digest)
	if err != nil {
		s.unlock(wOpts.Ref)
		return nil, err
	}

	return w, nil // lock is now held by w.
}

func (s *store) resumeStatus(ref string, total int64, digester digest.Digester) (content.Status, error) {
	path, _, data := s.ingestPaths(ref)
	status, err := s.status(path)
	if err != nil {
		return status, fmt.Errorf("failed reading status of resume write: %w", err)
	}
	if ref != status.Ref {
		// NOTE(stevvooe): This is fairly catastrophic. Either we have some
		// layout corruption or a hash collision for the ref key.
		return status, fmt.Errorf("ref key does not match: %v != %v", ref, status.Ref)
	}

	if total > 0 && status.Total > 0 && total != status.Total {
		return status, fmt.Errorf("provided total differs from status: %v != %v", total, status.Total)
	}

	//nolint:dupword
	// TODO(stevvooe): slow slow slow!!, send to goroutine or use resumable hashes
	fp, err := os.Open(data)
	if err != nil {
		return status, err
	}

	p := bufPool.Get().(*[]byte)
	status.Offset, err = io.CopyBuffer(digester.Hash(), fp, *p)
	bufPool.Put(p)
	fp.Close()
	return status, err
}

// writer provides the main implementation of the Writer method. The caller
// must hold the lock correctly and release on error if there is a problem.
func (s *store) writer(ctx context.Context, ref string, total int64, expected digest.Digest) (content.Writer, error) {
	// TODO(stevvooe): Need to actually store expected here. We have
	// code in the service that shouldn't be dealing with this.
	if expected != "" {
		p, err := s.blobPath(expected)
		if err != nil {
			return nil, fmt.Errorf("calculating expected blob path for writer: %w", err)
		}
		if _, err := os.Stat(p); err == nil {
			return nil, fmt.Errorf("content %v: %w", expected, errdefs.ErrAlreadyExists)
		}
	}

	path, refp, data := s.ingestPaths(ref)

	var (
		digester  = digest.Canonical.Digester()
		offset    int64
		startedAt time.Time
		updatedAt time.Time
	)

	foundValidIngest := false

	if err := s.ensureIngestRootOnce(); err != nil {
		return nil, err
	}

	// ensure that the ingest path has been created.
	if err := os.Mkdir(path, 0755); err != nil {
		if !os.IsExist(err) {
			return nil, err
		}
		status, err := s.resumeStatus(ref, total, digester)
		if err == nil {
			foundValidIngest = true
			updatedAt = status.UpdatedAt
			startedAt = status.StartedAt
			total = status.Total
			offset = status.Offset
		} else {
			log.G(ctx).Infof("failed to resume the status from path %s: %s. will recreate them", path, err.Error())
		}
	}

	if !foundValidIngest {
		startedAt = time.Now()
		updatedAt = startedAt

		// the ingest is new, we need to setup the target location.
		// write the ref to a file for later use
		if err := os.WriteFile(refp, []byte(ref), 0666); err != nil {
			return nil, err
		}

		if err := writeTimestampFile(filepath.Join(path, "startedat"), startedAt); err != nil {
			return nil, err
		}

		if err := writeTimestampFile(filepath.Join(path, "updatedat"), startedAt); err != nil {
			return nil, err
		}

		if total > 0 {
			if err := os.WriteFile(filepath.Join(path, "total"), []byte(fmt.Sprint(total)), 0666); err != nil {
				return nil, err
			}
		}
	}

	fp, err := os.OpenFile(data, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open data file: %w", err)
	}

	if _, err := fp.Seek(offset, io.SeekStart); err != nil {
		fp.Close()
		return nil, fmt.Errorf("could not seek to current write offset: %w", err)
	}

	return &writer{
		s:         s,
		fp:        fp,
		ref:       ref,
		path:      path,
		offset:    offset,
		total:     total,
		digester:  digester,
		startedAt: startedAt,
		updatedAt: updatedAt,
	}, nil
}

// Abort an active transaction keyed by ref. If the ingest is active, it will
// be cancelled. Any resources associated with the ingest will be cleaned.
func (s *store) Abort(ctx context.Context, ref string) error {
	root := s.ingestRoot(ref)
	if err := os.RemoveAll(root); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("ingest ref %q: %w", ref, errdefs.ErrNotFound)
		}

		return err
	}

	return nil
}

func (s *store) blobPath(dgst digest.Digest) (string, error) {
	if err := dgst.Validate(); err != nil {
		return "", fmt.Errorf("cannot calculate blob path from invalid digest: %v: %w", err, errdefs.ErrInvalidArgument)
	}

	return filepath.Join(s.root, "blobs", dgst.Algorithm().String(), dgst.Encoded()), nil
}

func (s *store) ingestRoot(ref string) string {
	// we take a digest of the ref to keep the ingest paths constant length.
	// Note that this is not the current or potential digest of incoming content.
	dgst := digest.FromString(ref)
	return filepath.Join(s.root, "ingest", dgst.Encoded())
}

// ingestPaths are returned. The paths are the following:
//
// - root: entire ingest directory
// - ref: name of the starting ref, must be unique
// - data: file where data is written
func (s *store) ingestPaths(ref string) (string, string, string) {
	var (
		fp = s.ingestRoot(ref)
		rp = filepath.Join(fp, "ref")
		dp = filepath.Join(fp, "data")
	)

	return fp, rp, dp
}

func (s *store) ensureIngestRoot() error {
	return os.MkdirAll(filepath.Join(s.root, "ingest"), 0777)
}

func readFileString(path string) (string, error) {
	p, err := os.ReadFile(path)
	return string(p), err
}

// readFileTimestamp reads a file with just a timestamp present.
func readFileTimestamp(p string) (time.Time, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			err = fmt.Errorf("%s: %w", err.Error(), errdefs.ErrNotFound)
		}
		return time.Time{}, err
	}

	var t time.Time
	if err := t.UnmarshalText(b); err != nil {
		return time.Time{}, fmt.Errorf("could not parse timestamp file %v: %w", p, err)
	}

	return t, nil
}

func writeTimestampFile(p string, t time.Time) error {
	b, err := t.MarshalText()
	if err != nil {
		return err
	}
	return writeToCompletion(p, b, 0666)
}

func writeToCompletion(path string, data []byte, mode os.FileMode) error {
	tmp := fmt.Sprintf("%s.tmp", path)
	f, err := os.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_SYNC, mode)
	if err != nil {
		return fmt.Errorf("create tmp file: %w", err)
	}
	_, err = f.Write(data)
	f.Close()
	if err != nil {
		return fmt.Errorf("write tmp file: %w", err)
	}
	err = os.Rename(tmp, path)
	if err != nil {
		return fmt.Errorf("rename tmp file: %w", err)
	}
	return nil
}
