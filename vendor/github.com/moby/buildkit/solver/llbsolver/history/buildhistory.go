package history

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/leases"
	cerrdefs "github.com/containerd/errdefs"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/cmd/buildkitd/config"
	"github.com/moby/buildkit/identity"
	containerdsnapshot "github.com/moby/buildkit/snapshot/containerd"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/db"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/moby/buildkit/util/iohelper"
	"github.com/moby/buildkit/util/leaseutil"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	recordsBucket = "_records"
	versionBucket = "_version"
)

type QueueOpt struct {
	DB             db.Transactor
	LeaseManager   *leaseutil.Manager
	ContentStore   *containerdsnapshot.Store
	CleanConfig    *config.HistoryConfig
	GarbageCollect func(context.Context) error
	GracefulStop   <-chan struct{}
}

type Queue struct {
	// mu protects active, refs and deleted maps
	mu            sync.Mutex
	initOnce      sync.Once
	opt           QueueOpt
	ps            *pubsub[*controlapi.BuildHistoryEvent]
	active        map[string]*controlapi.BuildHistoryRecord
	finalizers    map[string]*finalizer
	refs          map[string]int
	deleted       map[string]struct{}
	hContentStore *containerdsnapshot.Store
	hLeaseManager *leaseutil.Manager
}

// finalizer controls completion of saving traces for a
// record and making it immutable
type finalizer struct {
	trigger func()
	done    chan struct{}
}

type StatusImportResult struct {
	Descriptor        ocispecs.Descriptor
	NumCachedSteps    int
	NumCompletedSteps int
	NumTotalSteps     int
	NumWarnings       int
}

func NewQueue(opt QueueOpt) (*Queue, error) {
	if opt.CleanConfig == nil {
		opt.CleanConfig = &config.HistoryConfig{
			MaxAge:     config.Duration{Duration: 48 * time.Hour},
			MaxEntries: 50,
		}
	}
	h := &Queue{
		opt: opt,
		ps: &pubsub[*controlapi.BuildHistoryEvent]{
			m: map[*channel[*controlapi.BuildHistoryEvent]]struct{}{},
		},
		active:     map[string]*controlapi.BuildHistoryRecord{},
		refs:       map[string]int{},
		deleted:    map[string]struct{}{},
		finalizers: map[string]*finalizer{},
	}

	ns := h.opt.ContentStore.Namespace()
	// double check invalid configuration
	ns2 := h.opt.LeaseManager.Namespace()
	if ns != ns2 {
		return nil, errors.Errorf("invalid configuration: content store namespace %q does not match lease manager namespace %q", ns, ns2)
	}
	h.hContentStore = h.opt.ContentStore.WithNamespace(ns + "_history")
	h.hLeaseManager = h.opt.LeaseManager.WithNamespace(ns + "_history")

	// v2 migration: all records need to be on isolated containerd ns from rest of buildkit
	needsMigration := false
	if err := h.opt.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(versionBucket))
		if b != nil {
			v := b.Get([]byte("version"))
			if v != nil {
				vi, err := strconv.ParseInt(string(v), 10, 64)
				if err == nil && vi > 1 {
					return nil
				}
			}
		}
		needsMigration = true
		return nil
	}); err != nil {
		return nil, err
	}
	if needsMigration {
		if err := h.migrateV2(); err != nil {
			return nil, err
		}
	}

	go func() {
		h.clearOrphans()
		for {
			h.gc()
			time.Sleep(120 * time.Second)
		}
	}()

	go func() {
		<-h.opt.GracefulStop
		h.mu.Lock()
		defer h.mu.Unlock()
		// if active builds then close will happen in finalizer
		if len(h.finalizers) == 0 && len(h.active) == 0 {
			go h.ps.Close()
		}
	}()

	return h, nil
}

func (h *Queue) gc() error {
	var records []*controlapi.BuildHistoryRecord

	if err := h.opt.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(key, dt []byte) error {
			var br controlapi.BuildHistoryRecord
			if err := br.UnmarshalVT(dt); err != nil {
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
	if len(records) < int(h.opt.CleanConfig.MaxEntries) {
		return nil
	}

	// sort array by newest records first
	slices.SortFunc(records, func(a, b *controlapi.BuildHistoryRecord) int {
		return -a.CompletedAt.AsTime().Compare(b.CompletedAt.AsTime())
	})

	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for _, r := range records[h.opt.CleanConfig.MaxEntries:] {
		if now.Add(-h.opt.CleanConfig.MaxAge.Duration).After(r.CompletedAt.AsTime()) {
			if _, err := h.delete(r.Ref); err != nil {
				return err
			}
		}
	}

	return nil
}

func (h *Queue) clearOrphans() error {
	ctx := context.Background()
	var records []*controlapi.BuildHistoryRecord

	if err := h.opt.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(key, dt []byte) error {
			var br controlapi.BuildHistoryRecord
			if err := proto.Unmarshal(dt, &br); err != nil {
				return errors.Wrapf(err, "failed to unmarshal build record %s", key)
			}
			recs, err := h.hLeaseManager.ListResources(ctx, leases.Lease{ID: h.leaseID(string(key))})
			if (err != nil && cerrdefs.IsNotFound(err)) || len(recs) == 0 {
				records = append(records, &br)
			}
			return nil
		})
	}); err != nil {
		return err
	}

	if len(records) == 0 {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, r := range records {
		bklog.G(ctx).Warnf("deleting build record %s due to missing blobs", r.Ref)
		if _, err := h.delete(r.Ref); err != nil {
			return err
		}
	}

	return nil
}

func (h *Queue) delete(ref string) (bool, error) {
	if _, ok := h.refs[ref]; ok {
		h.deleted[ref] = struct{}{}
		return false, nil
	}
	delete(h.deleted, ref)
	h.ps.Send(&controlapi.BuildHistoryEvent{
		Type:   controlapi.BuildHistoryEventType_DELETED,
		Record: &controlapi.BuildHistoryRecord{Ref: ref},
	})
	if err := h.opt.DB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return errors.Wrapf(os.ErrNotExist, "failed to retrieve bucket %s", recordsBucket)
		}
		err1 := b.Delete([]byte(ref))
		err2 := h.hLeaseManager.Delete(context.TODO(), leases.Lease{ID: h.leaseID(ref)})
		if err1 != nil {
			return err1
		}
		return err2
	}); err != nil {
		return false, err
	}
	return true, nil
}

func (h *Queue) init() error {
	var err error
	h.initOnce.Do(func() {
		err = h.opt.DB.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(recordsBucket))
			return err
		})
	})
	return err
}

func (h *Queue) leaseID(id string) string {
	return "ref_" + id
}

func (h *Queue) addResource(ctx context.Context, l leases.Lease, desc *controlapi.Descriptor, detectSkipLayers bool) error {
	if desc == nil {
		return nil
	}
	if _, err := h.hContentStore.Info(ctx, digest.Digest(desc.Digest)); err != nil {
		if cerrdefs.IsNotFound(err) {
			lr, ctx, err := leaseutil.NewLease(ctx, h.hLeaseManager, leases.WithID("history_migration_"+identity.NewID()), leaseutil.MakeTemporary)
			if err != nil {
				return err
			}
			defer lr.Discard()
			ok, err := h.migrateBlobV2(ctx, desc.Digest, detectSkipLayers)
			if err != nil {
				return err
			}
			if !ok {
				return errors.Errorf("unknown blob %s in history", desc.Digest)
			}
		}
	}
	return h.hLeaseManager.AddResource(ctx, l, leases.Resource{
		ID:   desc.Digest,
		Type: "content",
	})
}

func (h *Queue) UpdateRef(ctx context.Context, ref string, upt func(r *controlapi.BuildHistoryRecord) error) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var br controlapi.BuildHistoryRecord
	if err := h.opt.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return errors.Wrapf(os.ErrNotExist, "failed to retrieve bucket %s", recordsBucket)
		}
		dt := b.Get([]byte(ref))
		if dt == nil {
			return errors.Wrapf(os.ErrNotExist, "failed to retrieve ref %s", ref)
		}

		if err := br.UnmarshalVT(dt); err != nil {
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

	if err := h.update(ctx, &br); err != nil {
		return err
	}
	h.ps.Send(&controlapi.BuildHistoryEvent{
		Type:   controlapi.BuildHistoryEventType_COMPLETE,
		Record: &br,
	})
	return nil
}

func (h *Queue) Status(ctx context.Context, ref string, st chan<- *client.SolveStatus) error {
	h.init()
	var br controlapi.BuildHistoryRecord
	if err := h.opt.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return errors.Wrapf(os.ErrNotExist, "failed to retrieve bucket %s", recordsBucket)
		}
		dt := b.Get([]byte(ref))
		if dt == nil {
			return errors.Wrapf(os.ErrNotExist, "failed to retrieve ref %s", ref)
		}

		if err := br.UnmarshalVT(dt); err != nil {
			return errors.Wrapf(err, "failed to unmarshal build record %s", ref)
		}
		return nil
	}); err != nil {
		return err
	}

	if br.Logs == nil {
		return nil
	}

	ra, err := h.hContentStore.ReaderAt(ctx, ocispecs.Descriptor{
		Digest:    digest.Digest(br.Logs.Digest),
		Size:      br.Logs.Size,
		MediaType: br.Logs.MediaType,
	})
	if err != nil {
		return err
	}

	rc := iohelper.ReadCloser(ra)
	defer rc.Close()

	brdr := bufio.NewReader(rc)

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
		if err := sr.UnmarshalVT(buf[:sz]); err != nil {
			return err
		}
		st <- client.NewSolveStatus(&sr)
	}

	return nil
}

func (h *Queue) update(ctx context.Context, rec *controlapi.BuildHistoryRecord) error {
	return h.opt.DB.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return nil
		}
		dt, err := rec.MarshalVT()
		if err != nil {
			return err
		}

		l, err := h.hLeaseManager.Create(ctx, leases.WithID(h.leaseID(rec.Ref)))
		created := true
		if err != nil {
			if !errors.Is(err, cerrdefs.ErrAlreadyExists) {
				return err
			}
			l = leases.Lease{ID: h.leaseID(rec.Ref)}
			created = false
		}

		defer func() {
			if err != nil && created {
				h.hLeaseManager.Delete(context.WithoutCancel(ctx), l)
			}
		}()

		if err := h.addResource(ctx, l, rec.Logs, false); err != nil {
			return err
		}
		if err := h.addResource(ctx, l, rec.Trace, false); err != nil {
			return err
		}
		if err := h.addResource(ctx, l, rec.ExternalError, false); err != nil {
			return err
		}
		if rec.Result != nil {
			if err := h.addResource(ctx, l, rec.Result.ResultDeprecated, true); err != nil {
				return err
			}
			for _, res := range rec.Result.Results {
				if err := h.addResource(ctx, l, res, true); err != nil {
					return err
				}
			}
			for _, att := range rec.Result.Attestations {
				if err := h.addResource(ctx, l, att, false); err != nil {
					return err
				}
			}
		}
		for _, r := range rec.Results {
			if err := h.addResource(ctx, l, r.ResultDeprecated, true); err != nil {
				return err
			}
			for _, res := range r.Results {
				if err := h.addResource(ctx, l, res, true); err != nil {
					return err
				}
			}
			for _, att := range r.Attestations {
				if err := h.addResource(ctx, l, att, false); err != nil {
					return err
				}
			}
		}

		return b.Put([]byte(rec.Ref), dt)
	})
}

func (h *Queue) AcquireFinalizer(ref string) (<-chan struct{}, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	trigger := make(chan struct{})
	f := &finalizer{
		trigger: sync.OnceFunc(func() {
			close(trigger)
		}),
		done: make(chan struct{}),
	}
	h.finalizers[ref] = f
	go func() {
		<-f.done
		h.mu.Lock()
		delete(h.finalizers, ref)
		// if gracefulstop then release listeners after finalize
		if len(h.finalizers) == 0 {
			select {
			case <-h.opt.GracefulStop:
				go h.ps.Close()
			default:
			}
		}
		h.mu.Unlock()
	}()
	return trigger, sync.OnceFunc(func() {
		close(f.done)
	})
}

func (h *Queue) Finalize(ctx context.Context, ref string) error {
	h.mu.Lock()
	f, ok := h.finalizers[ref]
	h.mu.Unlock()
	if !ok {
		return nil
	}
	f.trigger()
	<-f.done
	return nil
}

func (h *Queue) Update(ctx context.Context, e *controlapi.BuildHistoryEvent) error {
	h.init()
	h.mu.Lock()
	defer h.mu.Unlock()

	e = e.CloneVT()

	if e.Type == controlapi.BuildHistoryEventType_STARTED {
		h.active[e.Record.Ref] = e.Record
		h.ps.Send(e)
	}

	if e.Type == controlapi.BuildHistoryEventType_COMPLETE {
		delete(h.active, e.Record.Ref)
		if err := h.update(ctx, e.Record); err != nil {
			return err
		}
		h.ps.Send(e)
	}
	return nil
}

func (h *Queue) Delete(ctx context.Context, ref string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	v, err := h.delete(ref)
	if err != nil {
		return err
	}
	if v {
		return h.opt.GarbageCollect(ctx)
	}
	return nil
}

func (h *Queue) OpenBlobWriter(ctx context.Context, mt string) (_ *Writer, err error) {
	l, err := h.hLeaseManager.Create(ctx, leases.WithRandomID(), leases.WithExpiration(5*time.Minute), leaseutil.MakeTemporary)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			h.hLeaseManager.Delete(context.WithoutCancel(ctx), l)
		}
	}()

	ctx = leases.WithLease(ctx, l.ID)

	w, err := content.OpenWriter(ctx, h.hContentStore, content.WithRef("history-"+h.leaseID(l.ID)))
	if err != nil {
		return nil, err
	}

	return &Writer{
		mt:    mt,
		lm:    h.hLeaseManager,
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
	if err := w.w.Commit(leases.WithLease(ctx, w.l.ID), int64(w.sz), dgst); err != nil {
		if !cerrdefs.IsAlreadyExists(err) {
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

func (h *Queue) ImportError(ctx context.Context, err error) (_ *spb.Status, _ *controlapi.Descriptor, _ func(), retErr error) {
	st, ok := grpcerrors.AsGRPCStatus(grpcerrors.ToGRPC(ctx, err))
	if !ok {
		st = status.New(codes.Unknown, err.Error())
	}

	stpb := st.Proto()
	dt, err := proto.Marshal(stpb)
	if err != nil {
		return nil, nil, nil, err
	}

	w, err := h.OpenBlobWriter(ctx, "application/vnd.googeapis.google.rpc.status+proto")
	if err != nil {
		return nil, nil, nil, err
	}

	defer func() {
		if retErr != nil {
			w.Discard()
		}
	}()

	if _, err := w.Write(dt); err != nil {
		return nil, nil, nil, err
	}

	desc, release, err := w.Commit(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	// clear details part of the error that are saved to main record
	stpb.Details = nil

	return stpb, &controlapi.Descriptor{
		Digest:    string(desc.Digest),
		Size:      desc.Size,
		MediaType: desc.MediaType,
	}, release, nil
}

func (h *Queue) ImportStatus(ctx context.Context, ch chan *client.SolveStatus) (_ *StatusImportResult, _ func(), err error) {
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
	var numWarnings int

	buf := make([]byte, 32*1024)
	for st := range ch {
		numWarnings += len(st.Warnings)
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
			sz := pst.SizeVT()
			if len(buf) < sz {
				buf = make([]byte, sz)
			}
			n, err := pst.MarshalToVT(buf)
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
		NumWarnings:       numWarnings,
	}, release, nil
}

func (h *Queue) Listen(ctx context.Context, req *controlapi.BuildHistoryRequest, f func(*controlapi.BuildHistoryEvent) error) error {
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
					h.delete(req.Ref)
				}
			}
			h.mu.Unlock()
		}()
	}

	// make a copy of events for active builds so we don't keep a lock during grpc send
	actives := make([]*controlapi.BuildHistoryEvent, 0, len(h.active))

	for _, e := range h.active {
		if req.Ref != "" && e.Ref != req.Ref {
			continue
		}
		if _, ok := h.deleted[e.Ref]; ok {
			continue
		}
		actives = append(actives, &controlapi.BuildHistoryEvent{
			Type:   controlapi.BuildHistoryEventType_STARTED,
			Record: e,
		})
	}

	h.mu.Unlock()

	for _, e := range actives {
		if err := f(e); err != nil {
			return err
		}
	}

	if !req.ActiveOnly {
		events := []*controlapi.BuildHistoryEvent{}
		if err := h.opt.DB.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(recordsBucket))
			if b == nil {
				return nil
			}
			return b.ForEach(func(key, dt []byte) error {
				if req.Ref != "" && req.Ref != string(key) {
					return nil
				}
				var br controlapi.BuildHistoryRecord
				if err := br.UnmarshalVT(dt); err != nil {
					return errors.Wrapf(err, "failed to unmarshal build record %s", key)
				}
				events = append(events, &controlapi.BuildHistoryEvent{
					Record: &br,
					Type:   controlapi.BuildHistoryEventType_COMPLETE,
				})
				return nil
			})
		}); err != nil {
			return err
		}
		// filter out records that have been marked for deletion
		h.mu.Lock()
		for i, e := range events {
			if _, ok := h.deleted[e.Record.Ref]; ok {
				events[i] = nil
			}
		}
		h.mu.Unlock()

		events, err := filterHistoryEvents(events, req.Filter, req.Limit)
		if err != nil {
			return err
		}

		for _, e := range events {
			if e == nil || e.Record == nil {
				continue
			}
			if err := f(e); err != nil {
				return err
			}
		}
	}

	if req.EarlyExit {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
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
