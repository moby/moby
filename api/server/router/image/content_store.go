package image

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/leases"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/errdefs"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type contentStoreRouter struct {
	routes  []router.Route
	backend *contentBackend
}

type contentBackend struct {
	sessions map[string]*session
	super    content.Store
	lm       leases.Manager
	mut      sync.Mutex
}

func (cb *contentBackend) NewSession(ctx context.Context) (*session, error) {
	cb.mut.Lock()
	defer cb.mut.Unlock()

	lease, err := cb.lm.Create(ctx, leases.WithRandomID())
	if err != nil {
		return nil, fmt.Errorf("failed to create lease for session: %w", err)
	}

	sessionCtx, cancelCtx := context.WithCancel(context.Background())
	id := uuid.New().String()

	sessionCtx = leases.WithLease(sessionCtx, lease.ID)
	sess := &session{
		Id:        id,
		Super:     cb.super,
		Ctx:       sessionCtx,
		CancelCtx: cancelCtx,
		writers:   make(map[string]content.Writer),
		lease:     lease,
	}

	cb.sessions[id] = sess
	return sess, nil
}

func (cb *contentBackend) GetSession(_ context.Context, id string) (*session, error) {
	cb.mut.Lock()
	defer cb.mut.Unlock()

	sess, ok := cb.sessions[id]
	if !ok {
		return nil, errdefs.NotFound(fmt.Errorf("session %s not found", id))
	}

	return sess, nil
}

func (cb *contentBackend) NewWriter(ctx context.Context, sess *session, opts ...content.WriterOpt) (content.Writer, string, error) {
	var o content.WriterOpts
	for _, opt := range opts {
		_ = opt(&o)
	}

	cb.mut.Lock()
	defer cb.mut.Unlock()

	id := uuid.New().String()
	wr, err := sess.Super.Writer(sess.Ctx, opts...)
	if err != nil {
		return nil, "", err
	}

	sess.writers[id] = wr
	return wr, id, nil
}

func (cb *contentBackend) CloseWriter(ctx context.Context, wr content.Writer) {
	cb.mut.Lock()
	defer cb.mut.Unlock()

	for _, sess := range cb.sessions {
		for id, w := range sess.writers {
			if w == wr {
				delete(sess.writers, id)
				return
			}
		}
	}
}

func (cb *contentBackend) CloseSession(ctx context.Context, sess *session) error {
	cb.mut.Lock()
	defer cb.mut.Unlock()

	sess.CancelCtx()

	if _, ok := cb.sessions[sess.Id]; !ok {
		return errdefs.NotFound(fmt.Errorf("session %s not found", sess.Id))
	}

	for _, wr := range sess.writers {
		_ = wr.Close()
	}
	sess.writers = nil

	if err := cb.lm.Delete(ctx, sess.lease, leases.SynchronousDelete); err != nil {
		return fmt.Errorf("failed to delete session lease: %w", err)
	}

	delete(cb.sessions, sess.Id)
	return nil
}

type session struct {
	Id        string
	Super     content.Store
	Ctx       context.Context
	CancelCtx context.CancelFunc
	lease     leases.Lease

	writers map[string]content.Writer
}

// NewContentRouter initializes a new image router
func NewContentRouter(store content.Store, lm leases.Manager) router.Router {
	ir := &contentStoreRouter{
		backend: &contentBackend{
			sessions: make(map[string]*session),
			super:    store,
			lm:       lm,
		},
	}
	ir.initRoutes()
	return ir
}

// Routes returns the available routes to the image controller
func (cr *contentStoreRouter) Routes() []router.Route {
	return cr.routes
}

// initRoutes initializes the routes in the image router
func (cr *contentStoreRouter) initRoutes() {
	cr.routes = []router.Route{
		// POST
		router.NewPostRoute("/contentstore", cr.postContentStoreOpen),
		// DELETE
		router.NewPostRoute("/contentstore", cr.sessionMiddleware(cr.deleteContentStoreClose)),

		router.NewPostRoute("/contentstore/writer", cr.sessionMiddleware(cr.postWriterOpen)),
		router.NewPostRoute("/contentstore/writer/write", cr.writerMiddleware(cr.postWriterWrite)),
		router.NewPostRoute("/contentstore/writer/commit", cr.writerMiddleware(cr.postWriterCommit)),
		router.NewGetRoute("/contentstore/writer/digest", cr.writerMiddleware(cr.getWriterDigest)),
		router.NewGetRoute("/contentstore/writer/status", cr.writerMiddleware(cr.getWriterStatus)),
		router.NewDeleteRoute("/contentstore/writer", cr.writerMiddleware(cr.deleteWriterClose)),
		router.NewPostRoute("/contentstore/writer/truncate", cr.writerMiddleware(cr.postWriterTruncate)),

		// PUT
		router.NewPutRoute("/contentstore/write", cr.sessionMiddleware(cr.putContentStoreWrite)),

		// GET
		router.NewGetRoute("/contentstore/info", cr.sessionMiddleware(cr.getContentStoreInfo)),
		router.NewGetRoute("/contentstore/size", cr.sessionMiddleware(cr.getContentStoreSize)),
		router.NewGetRoute("/contentstore/read", cr.sessionMiddleware(cr.getContentStoreRead)),
	}
}

type sessionKey struct{}
type writerKey struct{}

func getSession(ctx context.Context) *session {
	return ctx.Value(sessionKey{}).(*session)
}

func getWriter(ctx context.Context) content.Writer {
	return ctx.Value(writerKey{}).(content.Writer)
}

func (cr *contentStoreRouter) sessionMiddleware(next httputils.APIFunc) httputils.APIFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		if err := r.ParseForm(); err != nil {
			return err
		}

		sessId := r.Form.Get("session")
		if sessId == "" {
			return errdefs.InvalidParameter(errors.New("session is required"))
		}

		sess, err := cr.backend.GetSession(ctx, sessId)
		if err != nil {
			return err
		}

		ctx = context.WithValue(ctx, sessionKey{}, sess)
		return next(ctx, w, r, vars)
	}
}

func (cr *contentStoreRouter) writerMiddleware(next httputils.APIFunc) httputils.APIFunc {
	return cr.sessionMiddleware(func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
		if err := r.ParseForm(); err != nil {
			return err
		}

		sess := getSession(ctx)
		wrId := r.Form.Get("writer")
		if wrId == "" {
			return errdefs.InvalidParameter(errors.New("writer is required"))
		}

		wr, ok := sess.writers[wrId]
		if !ok {
			return errdefs.NotFound(fmt.Errorf("writer %s not found", wrId))
		}

		ctx = context.WithValue(ctx, writerKey{}, wr)
		return next(ctx, w, r, vars)
	})
}

func (cr *contentStoreRouter) postWriterOpen(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	opts, err := cr.writerOpts(r)
	if err != nil {
		return err
	}

	sess := getSession(ctx)
	_, id, err := cr.backend.NewWriter(ctx, sess, opts...)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, map[string]string{"writer": id})
}

func (cr *contentStoreRouter) deleteWriterClose(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	wr := getWriter(ctx)
	if wr == nil {
		return errdefs.InvalidParameter(errors.New("writer is required"))
	}

	if err := wr.Close(); err != nil {
		return err
	}

	cr.backend.CloseWriter(ctx, wr)

	return nil
}

func (cr *contentStoreRouter) getWriterDigest(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	wr := getWriter(ctx)
	if wr == nil {
		return errdefs.InvalidParameter(errors.New("writer is required"))
	}

	return httputils.WriteJSON(w, http.StatusOK, map[string]string{"digest": wr.Digest().String()})
}

func (cr *contentStoreRouter) postWriterTruncate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	wr := getWriter(ctx)
	if wr == nil {
		return errdefs.InvalidParameter(errors.New("writer is required"))
	}

	size, err := strconv.ParseInt(r.Form.Get("size"), 10, 64)
	if err != nil {
		return errdefs.InvalidParameter(fmt.Errorf("invalid size: %v", err))
	}

	if err := wr.Truncate(size); err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, struct{}{})
}

func (cr *contentStoreRouter) postWriterCommit(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	wr := getWriter(ctx)
	if wr == nil {
		return errdefs.InvalidParameter(errors.New("writer is required"))
	}

	var size int64
	if s := r.Form.Get("size"); s != "" {
		s, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return errdefs.InvalidParameter(fmt.Errorf("invalid size: %v", err))
		}
		size = s
	}

	var expected digest.Digest
	if e := r.Form.Get("expected"); e != "" {
		d, err := digest.Parse(e)
		if err != nil {
			return errdefs.InvalidParameter(fmt.Errorf("invalid expected digest: %v", err))
		}
		expected = d
	}

	if err := wr.Commit(ctx, size, expected); err != nil {
		if ctx.Err() != nil {
			return errdefs.Cancelled(err)
		}
		if cerrdefs.IsAlreadyExists(err) {
			return errdefs.Conflict(err)
		}
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, struct{}{})
}

func (cr *contentStoreRouter) getWriterStatus(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	wr := getWriter(ctx)
	if wr == nil {
		return errdefs.InvalidParameter(errors.New("writer is required"))
	}

	status, err := wr.Status()
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, status)
}

func (cr *contentStoreRouter) postWriterWrite(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	wr := getWriter(ctx)
	if wr == nil {
		return errdefs.InvalidParameter(errors.New("writer is required"))
	}

	n, err := content.CopyReader(wr, r.Body)
	if err != nil {
		return err
	}

	if ctx.Err() != nil {
		return errdefs.Cancelled(err)
	}
	return httputils.WriteJSON(w, http.StatusOK, map[string]int64{"n": n})
}

func (cr *contentStoreRouter) postContentStoreOpen(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	sess, err := cr.backend.NewSession(ctx)
	if err != nil {
		return err
	}

	// TODO handle leases
	return httputils.WriteJSON(w, http.StatusOK, map[string]string{"session": sess.Id})
}

func (cr *contentStoreRouter) deleteContentStoreClose(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	sess := getSession(ctx)
	cr.backend.CloseSession(ctx, sess)
	return nil
}

func (cr *contentStoreRouter) putContentStoreWrite(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	sess := getSession(ctx)

	opts, err := cr.writerOpts(r)
	if err != nil {
		return err
	}

	ctx = leases.WithLease(ctx, sess.lease.ID)
	wr, _, err := cr.backend.NewWriter(ctx, sess, opts...)
	if err != nil {
		return err
	}
	defer wr.Close()

	if _, err := content.CopyReader(wr, r.Body); err != nil {
		return err
	}

	if err := wr.Commit(ctx, r.ContentLength, ""); err != nil {
		if cerrdefs.IsAlreadyExists(err) {
			return errdefs.Conflict(err)
		}
		return err
	}

	dgst := wr.Digest()
	return httputils.WriteJSON(w, http.StatusOK, map[string]string{"digest": dgst.String()})
}

func (cr *contentStoreRouter) writerOpts(r *http.Request) ([]content.WriterOpt, error) {
	var opts []content.WriterOpt
	if r := r.Form.Get("ref"); r != "" {
		opts = append(opts, content.WithRef(r))
	} else {
		return nil, errdefs.InvalidParameter(errors.New("ref is required"))
	}

	if d := r.Form.Get("descriptor"); d != "" {
		var desc ocispec.Descriptor
		if err := json.Unmarshal([]byte(d), &desc); err != nil {
			return nil, err
		}
		opts = append(opts, content.WithDescriptor(desc))
	}

	return opts, nil
}

func (cr *contentStoreRouter) getContentStoreRead(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	ociDesc, err := cr.readDescriptor(r)
	if err != nil {
		return err
	}

	sess := getSession(ctx)
	rds, err := content.BlobReadSeeker(ctx, sess.Super, ociDesc)
	if err != nil {
		return err
	}
	defer rds.Close()

	http.ServeContent(w, r, string(ociDesc.Digest), time.Now(), rds)
	return nil
}

func (cr *contentStoreRouter) readDescriptor(r *http.Request) (ocispec.Descriptor, error) {
	desc := r.Form.Get("descriptor")
	if desc == "" {
		return ocispec.Descriptor{}, errdefs.InvalidParameter(errors.New("descriptor must be provided"))
	}

	ociDesc := ocispec.Descriptor{}
	if err := json.Unmarshal([]byte(desc), &ociDesc); err != nil {
		return ocispec.Descriptor{}, err
	}
	return ociDesc, nil
}

func (cr *contentStoreRouter) getContentStoreSize(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	ociDesc, err := cr.readDescriptor(r)
	if err != nil {
		return err
	}

	sess := getSession(ctx)
	ra, err := sess.Super.ReaderAt(ctx, ociDesc)
	if err != nil {
		return err
	}

	size := ra.Size()
	_ = ra.Close()

	return httputils.WriteJSON(w, http.StatusOK, map[string]int64{"size": size})
}

func (cr *contentStoreRouter) getContentStoreInfo(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	dgst := r.Form.Get("digest")

	if dgst == "" {
		return errdefs.InvalidParameter(errors.New("digest is required"))
	}

	d, err := digest.Parse(dgst)
	if err != nil {
		return errdefs.InvalidParameter(fmt.Errorf("invalid digest: %v", err))
	}

	sess := getSession(ctx)
	info, err := sess.Super.Info(ctx, d)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusOK, info)
}
