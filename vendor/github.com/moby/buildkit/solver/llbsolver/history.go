package llbsolver

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/leases"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/cmd/buildkitd/config"
	"github.com/moby/buildkit/util/leaseutil"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
)

const (
	recordsBucket = "_records"
)

type HistoryQueueOpt struct {
	DB           *bolt.DB
	LeaseManager leases.Manager
	ContentStore content.Store
	CleanConfig  *config.HistoryConfig
}

type HistoryQueue struct {
	mu       sync.Mutex
	initOnce sync.Once
	HistoryQueueOpt
	ps      *pubsub[*controlapi.BuildHistoryEvent]
	active  map[string]*controlapi.BuildHistoryRecord
	refs    map[string]int
	deleted map[string]struct{}
}

type StatusImportResult struct {
	Descriptor        ocispecs.Descriptor
	NumCachedSteps    int
	NumCompletedSteps int
	NumTotalSteps     int
}

func NewHistoryQueue(opt HistoryQueueOpt) *HistoryQueue {
	if opt.CleanConfig == nil {
		opt.CleanConfig = &config.HistoryConfig{
			MaxAge:     int64((48 * time.Hour).Seconds()),
			MaxEntries: 50,
		}
	}
	h := &HistoryQueue{
		HistoryQueueOpt: opt,
		ps: &pubsub[*controlapi.BuildHistoryEvent]{
			m: map[*channel[*controlapi.BuildHistoryEvent]]struct{}{},
		},
		active:  map[string]*controlapi.BuildHistoryRecord{},
		refs:    map[string]int{},
		deleted: map[string]struct{}{},
	}

	go func() {
		for {
			h.gc()
			time.Sleep(120 * time.Second)
		}
	}()

	return h
}

func (h *HistoryQueue) gc() error {
	var records []*controlapi.BuildHistoryRecord

	if err := h.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(key, dt []byte) error {
			var br controlapi.BuildHistoryRecord
			if err := br.Unmarshal(dt); err != nil {
				return errors.Wrapf(err, "failed to unmarshal build record %s", key)
			}
			if br.Pinned {
				return nil
			}
			records = append(records, &br)
			return nil
		})
	}); err != nil {
		return err
	}

	// in order for record to get deleted by gc it exceed both maxentries and maxage criteria

	if len(records) < int(h.CleanConfig.MaxEntries) {
		return nil
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].CompletedAt.Before(*records[j].CompletedAt)
	})

	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for _, r := range records[h.CleanConfig.MaxEntries:] {
		if now.Add(time.Duration(h.CleanConfig.MaxAge) * -time.Second).After(*r.CompletedAt) {
			if err := h.delete(r.Ref, false); err != nil {
				return err
			}
		}
	}

	return nil
}

func (h *HistoryQueue) delete(ref string, sync bool) error {
	if _, ok := h.refs[ref]; ok {
		h.deleted[ref] = struct{}{}
		return nil
	}
	delete(h.deleted, ref)
	if err := h.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return os.ErrNotExist
		}
		err1 := b.Delete([]byte(ref))
		var opts []leases.DeleteOpt
		if sync {
			opts = append(opts, leases.SynchronousDelete)
		}
		err2 := h.LeaseManager.Delete(context.TODO(), leases.Lease{ID: h.leaseID(ref)}, opts...)
		if err1 != nil {
			return err1
		}
		return err2
	}); err != nil {
		return err
	}
	return nil
}

func (h *HistoryQueue) init() error {
	var err error
	h.initOnce.Do(func() {
		err = h.DB.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(recordsBucket))
			return err
		})
	})
	return err
}

func (h *HistoryQueue) leaseID(id string) string {
	return "ref_" + id
}

func (h *HistoryQueue) addResource(ctx context.Context, l leases.Lease, desc *controlapi.Descriptor) error {
	if desc == nil {
		return nil
	}
	return h.LeaseManager.AddResource(ctx, l, leases.Resource{
		ID:   string(desc.Digest),
		Type: "content",
	})
}

func (h *HistoryQueue) UpdateRef(ctx context.Context, ref string, upt func(r *controlapi.BuildHistoryRecord) error) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var br controlapi.BuildHistoryRecord
	if err := h.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return os.ErrNotExist
		}
		dt := b.Get([]byte(ref))
		if dt == nil {
			return os.ErrNotExist
		}

		if err := br.Unmarshal(dt); err != nil {
			return errors.Wrapf(err, "failed to unmarshal build record %s", ref)
		}
		return nil
	}); err != nil {
		return err
	}

	if err := upt(&br); err != nil {
		return err
	}
	br.Generation++

	if br.Ref != ref {
		return errors.Errorf("invalid ref change")
	}

	if err := h.update(ctx, br); err != nil {
		return err
	}
	h.ps.Send(&controlapi.BuildHistoryEvent{
		Type:   controlapi.BuildHistoryEventType_COMPLETE,
		Record: &br,
	})
	return nil
}

func (h *HistoryQueue) Status(ctx context.Context, ref string, st chan<- *client.SolveStatus) error {
	h.init()
	var br controlapi.BuildHistoryRecord
	if err := h.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return os.ErrNotExist
		}
		dt := b.Get([]byte(ref))
		if dt == nil {
			return os.ErrNotExist
		}

		if err := br.Unmarshal(dt); err != nil {
			return errors.Wrapf(err, "failed to unmarshal build record %s", ref)
		}
		return nil
	}); err != nil {
		return err
	}

	if br.Logs == nil {
		return nil
	}

	ra, err := h.ContentStore.ReaderAt(ctx, ocispecs.Descriptor{
		Digest:    br.Logs.Digest,
		Size:      br.Logs.Size_,
		MediaType: br.Logs.MediaType,
	})
	if err != nil {
		return err
	}
	defer ra.Close()

	brdr := bufio.NewReader(&reader{ReaderAt: ra})

	buf := make([]byte, 32*1024)

	for {
		_, err := io.ReadAtLeast(brdr, buf[:4], 4)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		sz := binary.LittleEndian.Uint32(buf[:4])
		if sz > uint32(len(buf)) {
			buf = make([]byte, sz)
		}
		_, err = io.ReadAtLeast(brdr, buf[:sz], int(sz))
		if err != nil {
			return err
		}
		var sr controlapi.StatusResponse
		if err := sr.Unmarshal(buf[:sz]); err != nil {
			return err
		}
		st <- client.NewSolveStatus(&sr)
	}

	return nil
}

func (h *HistoryQueue) update(ctx context.Context, rec controlapi.BuildHistoryRecord) error {
	return h.DB.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return nil
		}
		dt, err := rec.Marshal()
		if err != nil {
			return err
		}

		l, err := h.LeaseManager.Create(ctx, leases.WithID(h.leaseID(rec.Ref)))
		created := true
		if err != nil {
			if !errors.Is(err, errdefs.ErrAlreadyExists) {
				return err
			}
			l = leases.Lease{ID: h.leaseID(rec.Ref)}
			created = false
		}

		defer func() {
			if err != nil && created {
				h.LeaseManager.Delete(ctx, l)
			}
		}()

		if err := h.addResource(ctx, l, rec.Logs); err != nil {
			return err
		}
		if err := h.addResource(ctx, l, rec.Trace); err != nil {
			return err
		}
		if rec.Result != nil {
			if err := h.addResource(ctx, l, rec.Result.Result); err != nil {
				return err
			}
			for _, att := range rec.Result.Attestations {
				if err := h.addResource(ctx, l, att); err != nil {
					return err
				}
			}
		}
		for _, r := range rec.Results {
			if err := h.addResource(ctx, l, r.Result); err != nil {
				return err
			}
			for _, att := range r.Attestations {
				if err := h.addResource(ctx, l, att); err != nil {
					return err
				}
			}
		}

		return b.Put([]byte(rec.Ref), dt)
	})
}

func (h *HistoryQueue) Update(ctx context.Context, e *controlapi.BuildHistoryEvent) error {
	h.init()
	h.mu.Lock()
	defer h.mu.Unlock()

	if e.Type == controlapi.BuildHistoryEventType_STARTED {
		h.active[e.Record.Ref] = e.Record
		h.ps.Send(e)
	}

	if e.Type == controlapi.BuildHistoryEventType_COMPLETE {
		delete(h.active, e.Record.Ref)
		if err := h.update(ctx, *e.Record); err != nil {
			return err
		}
		h.ps.Send(e)
	}
	return nil
}

func (h *HistoryQueue) Delete(ctx context.Context, ref string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.delete(ref, true)
}

func (h *HistoryQueue) OpenBlobWriter(ctx context.Context, mt string) (_ *Writer, err error) {
	l, err := h.LeaseManager.Create(ctx, leases.WithRandomID(), leases.WithExpiration(5*time.Minute), leaseutil.MakeTemporary)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			h.LeaseManager.Delete(ctx, l)
		}
	}()

	ctx = leases.WithLease(ctx, l.ID)

	w, err := content.OpenWriter(ctx, h.ContentStore, content.WithRef("history-"+h.leaseID(l.ID)))
	if err != nil {
		return nil, err
	}

	return &Writer{
		mt:    mt,
		lm:    h.LeaseManager,
		l:     l,
		w:     w,
		dgstr: digest.Canonical.Digester(),
	}, nil
}

type Writer struct {
	mt string
	w  content.Writer
	lm leases.Manager
	l  leases.Lease

	dgstr digest.Digester
	sz    int
}

func (w *Writer) Write(p []byte) (int, error) {
	if _, err := w.dgstr.Hash().Write(p); err != nil {
		return 0, err
	}
	w.sz += len(p)
	return w.w.Write(p)
}

func (w *Writer) Discard() {
	w.w.Close()
	w.lm.Delete(context.TODO(), w.l)
}

func (w *Writer) Commit(ctx context.Context) (*ocispecs.Descriptor, func(), error) {
	dgst := w.dgstr.Digest()
	sz := int64(w.sz)
	if err := w.w.Commit(ctx, int64(w.sz), dgst); err != nil {
		if !errdefs.IsAlreadyExists(err) {
			w.Discard()
			return nil, nil, err
		}
	}
	return &ocispecs.Descriptor{
			MediaType: w.mt,
			Digest:    dgst,
			Size:      sz,
		},
		func() {
			w.lm.Delete(context.TODO(), w.l)
		}, nil
}

func (h *HistoryQueue) ImportStatus(ctx context.Context, ch chan *client.SolveStatus) (_ *StatusImportResult, _ func(), err error) {
	defer func() {
		if ch == nil {
			return
		}
		for range ch {
		}
	}()

	w, err := h.OpenBlobWriter(ctx, "application/vnd.buildkit.status.v0")
	if err != nil {
		return nil, nil, err
	}

	bufW := bufio.NewWriter(w)

	defer func() {
		if err != nil {
			w.Discard()
		}
	}()

	type vtxInfo struct {
		cached    bool
		completed bool
	}
	vtxMap := make(map[digest.Digest]*vtxInfo)

	buf := make([]byte, 32*1024)
	for st := range ch {
		for _, vtx := range st.Vertexes {
			if _, ok := vtxMap[vtx.Digest]; !ok {
				vtxMap[vtx.Digest] = &vtxInfo{}
			}
			if vtx.Cached {
				vtxMap[vtx.Digest].cached = true
			}
			if vtx.Completed != nil {
				vtxMap[vtx.Digest].completed = true
			}
		}

		hdr := make([]byte, 4)
		for _, pst := range st.Marshal() {
			sz := pst.Size()
			if len(buf) < sz {
				buf = make([]byte, sz)
			}
			n, err := pst.MarshalTo(buf)
			if err != nil {
				return nil, nil, err
			}
			binary.LittleEndian.PutUint32(hdr, uint32(n))
			if _, err := bufW.Write(hdr); err != nil {
				return nil, nil, err
			}
			if _, err := bufW.Write(buf[:n]); err != nil {
				return nil, nil, err
			}
		}
	}
	if err := bufW.Flush(); err != nil {
		return nil, nil, err
	}
	desc, release, err := w.Commit(ctx)
	if err != nil {
		return nil, nil, err
	}

	numCached := 0
	numCompleted := 0
	for _, info := range vtxMap {
		if info.cached {
			numCached++
		}
		if info.completed {
			numCompleted++
		}
	}

	return &StatusImportResult{
		Descriptor:        *desc,
		NumCachedSteps:    numCached,
		NumCompletedSteps: numCompleted,
		NumTotalSteps:     len(vtxMap),
	}, release, nil
}

func (h *HistoryQueue) Listen(ctx context.Context, req *controlapi.BuildHistoryRequest, f func(*controlapi.BuildHistoryEvent) error) error {
	h.init()

	h.mu.Lock()
	sub := h.ps.Subscribe()
	defer sub.close()

	if req.Ref != "" {
		if _, ok := h.deleted[req.Ref]; ok {
			h.mu.Unlock()
			return errors.Wrapf(os.ErrNotExist, "ref %s is deleted", req.Ref)
		}

		h.refs[req.Ref]++
		defer func() {
			h.mu.Lock()
			h.refs[req.Ref]--
			if _, ok := h.deleted[req.Ref]; ok {
				if h.refs[req.Ref] == 0 {
					delete(h.refs, req.Ref)
					h.delete(req.Ref, false)
				}
			}
			h.mu.Unlock()
		}()
	}

	for _, e := range h.active {
		if req.Ref != "" && e.Ref != req.Ref {
			continue
		}
		sub.ps.Send(&controlapi.BuildHistoryEvent{
			Type:   controlapi.BuildHistoryEventType_STARTED,
			Record: e,
		})
	}

	h.mu.Unlock()

	if !req.ActiveOnly {
		if err := h.DB.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(recordsBucket))
			if b == nil {
				return nil
			}
			return b.ForEach(func(key, dt []byte) error {
				if req.Ref != "" && req.Ref != string(key) {
					return nil
				}
				var br controlapi.BuildHistoryRecord
				if err := br.Unmarshal(dt); err != nil {
					return errors.Wrapf(err, "failed to unmarshal build record %s", key)
				}
				if err := f(&controlapi.BuildHistoryEvent{
					Record: &br,
					Type:   controlapi.BuildHistoryEventType_COMPLETE,
				}); err != nil {
					return err
				}
				return nil
			})
		}); err != nil {
			return err
		}
	}

	if req.EarlyExit {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e := <-sub.ch:
			if req.Ref != "" && req.Ref != e.Record.Ref {
				continue
			}
			if err := f(e); err != nil {
				return err
			}
		case <-sub.done:
			return nil
		}
	}
}

type pubsub[T any] struct {
	mu sync.Mutex
	m  map[*channel[T]]struct{}
}

func (p *pubsub[T]) Subscribe() *channel[T] {
	p.mu.Lock()
	c := &channel[T]{
		ps:   p,
		ch:   make(chan T, 32),
		done: make(chan struct{}),
	}
	p.m[c] = struct{}{}
	p.mu.Unlock()
	return c
}

func (p *pubsub[T]) Send(v T) {
	p.mu.Lock()
	for c := range p.m {
		go c.send(v)
	}
	p.mu.Unlock()
}

type channel[T any] struct {
	ps        *pubsub[T]
	ch        chan T
	done      chan struct{}
	closeOnce sync.Once
}

func (p *channel[T]) send(v T) {
	select {
	case p.ch <- v:
	case <-p.done:
	}
}

func (p *channel[T]) close() {
	p.closeOnce.Do(func() {
		p.ps.mu.Lock()
		delete(p.ps.m, p)
		p.ps.mu.Unlock()
		close(p.done)
	})
}

type reader struct {
	io.ReaderAt
	pos int64
}

func (r *reader) Read(p []byte) (int, error) {
	n, err := r.ReaderAt.ReadAt(p, r.pos)
	r.pos += int64(len(p))
	return n, err
}
