package llbsolver

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/leases"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/gogo/googleapis/google/rpc"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/cmd/buildkitd/config"
	"github.com/moby/buildkit/identity"
	containerdsnapshot "github.com/moby/buildkit/snapshot/containerd"
	"github.com/moby/buildkit/util/db"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/moby/buildkit/util/iohelper"
	"github.com/moby/buildkit/util/leaseutil"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	recordsBucket = "_records"
	versionBucket = "_version"
)

type HistoryQueueOpt struct {
	DB           db.Transactor
	LeaseManager *leaseutil.Manager
	ContentStore *containerdsnapshot.Store
	CleanConfig  *config.HistoryConfig
}

type HistoryQueue struct {
	// mu protects active, refs and deleted maps
	mu            sync.Mutex
	initOnce      sync.Once
	opt           HistoryQueueOpt
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

func NewHistoryQueue(opt HistoryQueueOpt) (*HistoryQueue, error) {
	if opt.CleanConfig == nil {
		opt.CleanConfig = &config.HistoryConfig{
			MaxAge:     config.Duration{Duration: 48 * time.Hour},
			MaxEntries: 50,
		}
	}
	h := &HistoryQueue{
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
		for {
			h.gc()
			time.Sleep(120 * time.Second)
		}
	}()

	return h, nil
}

func (h *HistoryQueue) migrateV2() error {
	ctx := context.Background()

	if err := h.opt.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return nil
		}
		ctx, release, err := leaseutil.WithLease(ctx, h.hLeaseManager, leases.WithID("history_migration_"+identity.NewID()), leaseutil.MakeTemporary)
		if err != nil {
			return err
		}
		defer release(context.WithoutCancel(ctx))
		return b.ForEach(func(key, dt []byte) error {
			recs, err := h.opt.LeaseManager.ListResources(ctx, leases.Lease{ID: h.leaseID(string(key))})
			if err != nil {
				if cerrdefs.IsNotFound(err) {
					return nil
				}
				return err
			}
			recs2 := make([]leases.Resource, 0, len(recs))
			for _, r := range recs {
				if r.Type == "content" {
					if ok, err := h.migrateBlobV2(ctx, r.ID, false); err != nil {
						return err
					} else if ok {
						recs2 = append(recs2, r)
					}
				} else {
					return errors.Errorf("unknown resource type %q", r.Type)
				}
			}

			l, err := h.hLeaseManager.Create(ctx, leases.WithID(h.leaseID(string(key))))
			if err != nil {
				if !errors.Is(err, cerrdefs.ErrAlreadyExists) {
					return err
				}
				l = leases.Lease{ID: string(key)}
			}

			for _, r := range recs2 {
				if err := h.hLeaseManager.AddResource(ctx, l, r); err != nil {
					return err
				}
			}

			return h.opt.LeaseManager.Delete(ctx, leases.Lease{ID: h.leaseID(string(key))})
		})
	}); err != nil {
		return err
	}

	if err := h.opt.DB.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(versionBucket))
		if err != nil {
			return err
		}
		return b.Put([]byte("version"), []byte("2"))
	}); err != nil {
		return err
	}

	return nil
}

func (h *HistoryQueue) blobRefs(ctx context.Context, dgst digest.Digest, detectSkipLayer bool) ([]digest.Digest, error) {
	info, err := h.opt.ContentStore.Info(ctx, dgst)
	if err != nil {
		return nil, err // allow missing blobs
	}
	var out []digest.Digest
	layers := map[digest.Digest]struct{}{}
	if detectSkipLayer {
		dt, err := content.ReadBlob(ctx, h.opt.ContentStore, ocispecs.Descriptor{
			Digest: dgst,
		})
		if err != nil {
			return nil, err
		}
		var mfst ocispecs.Manifest
		if err := json.Unmarshal(dt, &mfst); err != nil {
			return nil, err
		}
		for _, l := range mfst.Layers {
			layers[l.Digest] = struct{}{}
		}
	}
	for k, v := range info.Labels {
		if !strings.HasPrefix(k, "containerd.io/gc.ref.content.") {
			continue
		}
		dgst, err := digest.Parse(v)
		if err != nil {
			continue
		}
		if _, ok := layers[dgst]; ok {
			continue
		}
		out = append(out, dgst)
	}
	return out, nil
}

func (h *HistoryQueue) migrateBlobV2(ctx context.Context, id string, detectSkipLayers bool) (bool, error) {
	dgst, err := digest.Parse(id)
	if err != nil {
		return false, err
	}

	refs, _ := h.blobRefs(ctx, dgst, detectSkipLayers) // allow missing blobs
	labels := map[string]string{}
	for i, r := range refs {
		labels["containerd.io/gc.ref.content."+strconv.Itoa(i)] = r.String()
	}

	w, err := content.OpenWriter(ctx, h.hContentStore, content.WithDescriptor(ocispecs.Descriptor{
		Digest: dgst,
	}), content.WithRef("history-migrate-"+id))
	if err != nil {
		if cerrdefs.IsAlreadyExists(err) {
			return true, nil
		}
		return false, err
	}
	defer w.Close()
	ra, err := h.opt.ContentStore.ReaderAt(ctx, ocispecs.Descriptor{
		Digest: dgst,
	})
	if err != nil {
		return false, nil // allow skipping
	}
	defer ra.Close()
	if err := content.Copy(ctx, w, iohelper.ReadCloser(ra), 0, dgst, content.WithLabels(labels)); err != nil {
		return false, err
	}

	for _, refs := range refs {
		h.migrateBlobV2(ctx, refs.String(), detectSkipLayers) // allow missing blobs
	}

	return true, nil
}

func (h *HistoryQueue) gc() error {
	var records []*controlapi.BuildHistoryRecord

	if err := h.opt.DB.View(func(tx *bolt.Tx) error {
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
	if len(records) < int(h.opt.CleanConfig.MaxEntries) {
		return nil
	}

	// sort array by newest records first
	sort.Slice(records, func(i, j int) bool {
		return records[i].CompletedAt.After(*records[j].CompletedAt)
	})

	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for _, r := range records[h.opt.CleanConfig.MaxEntries:] {
		if now.Add(-h.opt.CleanConfig.MaxAge.Duration).After(*r.CompletedAt) {
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
		var opts []leases.DeleteOpt
		if sync {
			opts = append(opts, leases.SynchronousDelete)
		}
		err2 := h.hLeaseManager.Delete(context.TODO(), leases.Lease{ID: h.leaseID(ref)}, opts...)
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
		err = h.opt.DB.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(recordsBucket))
			return err
		})
	})
	return err
}

func (h *HistoryQueue) leaseID(id string) string {
	return "ref_" + id
}

func (h *HistoryQueue) addResource(ctx context.Context, l leases.Lease, desc *controlapi.Descriptor, detectSkipLayers bool) error {
	if desc == nil {
		return nil
	}
	if _, err := h.hContentStore.Info(ctx, desc.Digest); err != nil {
		if cerrdefs.IsNotFound(err) {
			lr, ctx, err := leaseutil.NewLease(ctx, h.hLeaseManager, leases.WithID("history_migration_"+identity.NewID()), leaseutil.MakeTemporary)
			if err != nil {
				return err
			}
			defer lr.Discard()
			ok, err := h.migrateBlobV2(ctx, string(desc.Digest), detectSkipLayers)
			if err != nil {
				return err
			}
			if !ok {
				return errors.Errorf("unknown blob %s in history", desc.Digest)
			}
		}
	}
	return h.hLeaseManager.AddResource(ctx, l, leases.Resource{
		ID:   string(desc.Digest),
		Type: "content",
	})
}

func (h *HistoryQueue) UpdateRef(ctx context.Context, ref string, upt func(r *controlapi.BuildHistoryRecord) error) error {
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
	if err := h.opt.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return errors.Wrapf(os.ErrNotExist, "failed to retrieve bucket %s", recordsBucket)
		}
		dt := b.Get([]byte(ref))
		if dt == nil {
			return errors.Wrapf(os.ErrNotExist, "failed to retrieve ref %s", ref)
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

	ra, err := h.hContentStore.ReaderAt(ctx, ocispecs.Descriptor{
		Digest:    br.Logs.Digest,
		Size:      br.Logs.Size_,
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
		if err := sr.Unmarshal(buf[:sz]); err != nil {
			return err
		}
		st <- client.NewSolveStatus(&sr)
	}

	return nil
}

func (h *HistoryQueue) update(ctx context.Context, rec controlapi.BuildHistoryRecord) error {
	return h.opt.DB.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(recordsBucket))
		if b == nil {
			return nil
		}
		dt, err := rec.Marshal()
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

func (h *HistoryQueue) AcquireFinalizer(ref string) (<-chan struct{}, func()) {
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
		h.mu.Unlock()
	}()
	return trigger, sync.OnceFunc(func() {
		close(f.done)
	})
}

func (h *HistoryQueue) Finalize(ctx context.Context, ref string) error {
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

func (h *HistoryQueue) ImportError(ctx context.Context, err error) (_ *rpc.Status, _ *controlapi.Descriptor, _ func(), retErr error) {
	st, ok := grpcerrors.AsGRPCStatus(grpcerrors.ToGRPC(ctx, err))
	if !ok {
		st = status.New(codes.Unknown, err.Error())
	}
	rpcStatus := grpcerrors.ToRPCStatus(st.Proto())

	dt, err := rpcStatus.Marshal()
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
	rpcStatus.Details = nil

	return rpcStatus, &controlapi.Descriptor{
		Digest:    desc.Digest,
		Size_:     desc.Size,
		MediaType: desc.MediaType,
	}, release, nil
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
		NumWarnings:       numWarnings,
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
				if err := br.Unmarshal(dt); err != nil {
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
