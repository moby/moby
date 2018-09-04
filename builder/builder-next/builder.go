package buildkit

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/libnetwork"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/control"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver/llbsolver"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/moby/buildkit/util/tracing"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	grpcmetadata "google.golang.org/grpc/metadata"
)

var errMultipleFilterValues = errors.New("filters expect only one value")

var cacheFields = map[string]bool{
	"id":          true,
	"parent":      true,
	"type":        true,
	"description": true,
	"inuse":       true,
	"shared":      true,
	"private":     true,
	// fields from buildkit that are not exposed
	"mutable":   false,
	"immutable": false,
}

func init() {
	llbsolver.AllowNetworkHostUnstable = true
}

// Opt is option struct required for creating the builder
type Opt struct {
	SessionManager    *session.Manager
	Root              string
	NetnsRoot         string
	Dist              images.DistributionServices
	NetworkController libnetwork.NetworkController
}

// Builder can build using BuildKit backend
type Builder struct {
	controller     *control.Controller
	reqBodyHandler *reqBodyHandler

	mu   sync.Mutex
	jobs map[string]*buildJob
}

// New creates a new builder
func New(opt Opt) (*Builder, error) {
	reqHandler := newReqBodyHandler(tracing.DefaultTransport)

	c, err := newController(reqHandler, opt)
	if err != nil {
		return nil, err
	}
	b := &Builder{
		controller:     c,
		reqBodyHandler: reqHandler,
		jobs:           map[string]*buildJob{},
	}
	return b, nil
}

// Cancel cancels a build using ID
func (b *Builder) Cancel(ctx context.Context, id string) error {
	b.mu.Lock()
	if j, ok := b.jobs[id]; ok && j.cancel != nil {
		j.cancel()
	}
	b.mu.Unlock()
	return nil
}

// DiskUsage returns a report about space used by build cache
func (b *Builder) DiskUsage(ctx context.Context) ([]*types.BuildCache, error) {
	duResp, err := b.controller.DiskUsage(ctx, &controlapi.DiskUsageRequest{})
	if err != nil {
		return nil, err
	}

	var items []*types.BuildCache
	for _, r := range duResp.Record {
		items = append(items, &types.BuildCache{
			ID:          r.ID,
			Parent:      r.Parent,
			Type:        r.RecordType,
			Description: r.Description,
			InUse:       r.InUse,
			Shared:      r.Shared,
			Size:        r.Size_,
			CreatedAt:   r.CreatedAt,
			LastUsedAt:  r.LastUsedAt,
			UsageCount:  int(r.UsageCount),
		})
	}
	return items, nil
}

// Prune clears all reclaimable build cache
func (b *Builder) Prune(ctx context.Context, opts types.BuildCachePruneOptions) (int64, []string, error) {
	ch := make(chan *controlapi.UsageRecord)

	eg, ctx := errgroup.WithContext(ctx)

	validFilters := make(map[string]bool, 1+len(cacheFields))
	validFilters["unused-for"] = true
	for k, v := range cacheFields {
		validFilters[k] = v
	}
	if err := opts.Filters.Validate(validFilters); err != nil {
		return 0, nil, err
	}

	var unusedFor time.Duration
	unusedForValues := opts.Filters.Get("unused-for")

	switch len(unusedForValues) {
	case 0:

	case 1:
		var err error
		unusedFor, err = time.ParseDuration(unusedForValues[0])
		if err != nil {
			return 0, nil, errors.Wrap(err, "unused-for filter expects a duration (e.g., '24h')")
		}

	default:
		return 0, nil, errMultipleFilterValues
	}

	bkFilter := make([]string, 0, opts.Filters.Len())
	for cacheField := range cacheFields {
		values := opts.Filters.Get(cacheField)
		switch len(values) {
		case 0:
			bkFilter = append(bkFilter, cacheField)
		case 1:
			bkFilter = append(bkFilter, cacheField+"=="+values[0])
		default:
			return 0, nil, errMultipleFilterValues
		}
	}

	eg.Go(func() error {
		defer close(ch)
		return b.controller.Prune(&controlapi.PruneRequest{
			All:          opts.All,
			KeepDuration: int64(unusedFor),
			KeepBytes:    opts.KeepStorage,
			Filter:       bkFilter,
		}, &pruneProxy{
			streamProxy: streamProxy{ctx: ctx},
			ch:          ch,
		})
	})

	var size int64
	var cacheIDs []string
	eg.Go(func() error {
		for r := range ch {
			size += r.Size_
			cacheIDs = append(cacheIDs, r.ID)
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return 0, nil, err
	}

	return size, cacheIDs, nil
}

// Build executes a build request
func (b *Builder) Build(ctx context.Context, opt backend.BuildConfig) (*builder.Result, error) {
	var rc = opt.Source

	if buildID := opt.Options.BuildID; buildID != "" {
		b.mu.Lock()

		upload := false
		if strings.HasPrefix(buildID, "upload-request:") {
			upload = true
			buildID = strings.TrimPrefix(buildID, "upload-request:")
		}

		if _, ok := b.jobs[buildID]; !ok {
			b.jobs[buildID] = newBuildJob()
		}
		j := b.jobs[buildID]
		var cancel func()
		ctx, cancel = context.WithCancel(ctx)
		j.cancel = cancel
		b.mu.Unlock()

		if upload {
			ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			err := j.SetUpload(ctx2, rc)
			return nil, err
		}

		if remoteContext := opt.Options.RemoteContext; remoteContext == "upload-request" {
			ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			var err error
			rc, err = j.WaitUpload(ctx2)
			if err != nil {
				return nil, err
			}
			opt.Options.RemoteContext = ""
		}

		defer func() {
			delete(b.jobs, buildID)
		}()
	}

	var out builder.Result

	id := identity.NewID()

	frontendAttrs := map[string]string{}

	if opt.Options.Target != "" {
		frontendAttrs["target"] = opt.Options.Target
	}

	if opt.Options.Dockerfile != "" && opt.Options.Dockerfile != "." {
		frontendAttrs["filename"] = opt.Options.Dockerfile
	}

	if opt.Options.RemoteContext != "" {
		if opt.Options.RemoteContext != "client-session" {
			frontendAttrs["context"] = opt.Options.RemoteContext
		}
	} else {
		url, cancel := b.reqBodyHandler.newRequest(rc)
		defer cancel()
		frontendAttrs["context"] = url
	}

	cacheFrom := append([]string{}, opt.Options.CacheFrom...)

	frontendAttrs["cache-from"] = strings.Join(cacheFrom, ",")

	for k, v := range opt.Options.BuildArgs {
		if v == nil {
			continue
		}
		frontendAttrs["build-arg:"+k] = *v
	}

	for k, v := range opt.Options.Labels {
		frontendAttrs["label:"+k] = v
	}

	if opt.Options.NoCache {
		frontendAttrs["no-cache"] = ""
	}

	if opt.Options.PullParent {
		frontendAttrs["image-resolve-mode"] = "pull"
	} else {
		frontendAttrs["image-resolve-mode"] = "default"
	}

	if opt.Options.Platform != "" {
		// same as in newBuilder in builder/dockerfile.builder.go
		// TODO: remove once opt.Options.Platform is of type specs.Platform
		sp, err := platforms.Parse(opt.Options.Platform)
		if err != nil {
			return nil, err
		}
		if err := system.ValidatePlatform(sp); err != nil {
			return nil, err
		}
		frontendAttrs["platform"] = opt.Options.Platform
	}

	switch opt.Options.NetworkMode {
	case "host", "none":
		frontendAttrs["force-network-mode"] = opt.Options.NetworkMode
	case "", "default":
	default:
		return nil, errors.Errorf("network mode %q not supported by buildkit", opt.Options.NetworkMode)
	}

	extraHosts, err := toBuildkitExtraHosts(opt.Options.ExtraHosts)
	if err != nil {
		return nil, err
	}
	frontendAttrs["add-hosts"] = extraHosts

	exporterAttrs := map[string]string{}

	if len(opt.Options.Tags) > 0 {
		exporterAttrs["name"] = strings.Join(opt.Options.Tags, ",")
	}

	req := &controlapi.SolveRequest{
		Ref:           id,
		Exporter:      "moby",
		ExporterAttrs: exporterAttrs,
		Frontend:      "dockerfile.v0",
		FrontendAttrs: frontendAttrs,
		Session:       opt.Options.SessionID,
	}

	if opt.Options.NetworkMode == "host" {
		req.Entitlements = append(req.Entitlements, entitlements.EntitlementNetworkHost)
	}

	aux := streamformatter.AuxFormatter{Writer: opt.ProgressWriter.Output}

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		resp, err := b.controller.Solve(ctx, req)
		if err != nil {
			return err
		}
		id, ok := resp.ExporterResponse["containerimage.digest"]
		if !ok {
			return errors.Errorf("missing image id")
		}
		out.ImageID = id
		return aux.Emit("moby.image.id", types.BuildResult{ID: id})
	})

	ch := make(chan *controlapi.StatusResponse)

	eg.Go(func() error {
		defer close(ch)
		// streamProxy.ctx is not set to ctx because when request is cancelled,
		// only the build request has to be cancelled, not the status request.
		stream := &statusProxy{streamProxy: streamProxy{ctx: context.TODO()}, ch: ch}
		return b.controller.Status(&controlapi.StatusRequest{Ref: id}, stream)
	})

	eg.Go(func() error {
		for sr := range ch {
			dt, err := sr.Marshal()
			if err != nil {
				return err
			}
			if err := aux.Emit("moby.buildkit.trace", dt); err != nil {
				return err
			}
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return &out, nil
}

type streamProxy struct {
	ctx context.Context
}

func (sp *streamProxy) SetHeader(_ grpcmetadata.MD) error {
	return nil
}

func (sp *streamProxy) SendHeader(_ grpcmetadata.MD) error {
	return nil
}

func (sp *streamProxy) SetTrailer(_ grpcmetadata.MD) {
}

func (sp *streamProxy) Context() context.Context {
	return sp.ctx
}
func (sp *streamProxy) RecvMsg(m interface{}) error {
	return io.EOF
}

type statusProxy struct {
	streamProxy
	ch chan *controlapi.StatusResponse
}

func (sp *statusProxy) Send(resp *controlapi.StatusResponse) error {
	return sp.SendMsg(resp)
}
func (sp *statusProxy) SendMsg(m interface{}) error {
	if sr, ok := m.(*controlapi.StatusResponse); ok {
		sp.ch <- sr
	}
	return nil
}

type pruneProxy struct {
	streamProxy
	ch chan *controlapi.UsageRecord
}

func (sp *pruneProxy) Send(resp *controlapi.UsageRecord) error {
	return sp.SendMsg(resp)
}
func (sp *pruneProxy) SendMsg(m interface{}) error {
	if sr, ok := m.(*controlapi.UsageRecord); ok {
		sp.ch <- sr
	}
	return nil
}

type contentStoreNoLabels struct {
	content.Store
}

func (c *contentStoreNoLabels) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	return content.Info{}, nil
}

type wrapRC struct {
	io.ReadCloser
	once   sync.Once
	err    error
	waitCh chan struct{}
}

func (w *wrapRC) Read(b []byte) (int, error) {
	n, err := w.ReadCloser.Read(b)
	if err != nil {
		e := err
		if e == io.EOF {
			e = nil
		}
		w.close(e)
	}
	return n, err
}

func (w *wrapRC) Close() error {
	err := w.ReadCloser.Close()
	w.close(err)
	return err
}

func (w *wrapRC) close(err error) {
	w.once.Do(func() {
		w.err = err
		close(w.waitCh)
	})
}

func (w *wrapRC) wait() error {
	<-w.waitCh
	return w.err
}

type buildJob struct {
	cancel func()
	waitCh chan func(io.ReadCloser) error
}

func newBuildJob() *buildJob {
	return &buildJob{waitCh: make(chan func(io.ReadCloser) error)}
}

func (j *buildJob) WaitUpload(ctx context.Context) (io.ReadCloser, error) {
	done := make(chan struct{})

	var upload io.ReadCloser
	fn := func(rc io.ReadCloser) error {
		w := &wrapRC{ReadCloser: rc, waitCh: make(chan struct{})}
		upload = w
		close(done)
		return w.wait()
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case j.waitCh <- fn:
		<-done
		return upload, nil
	}
}

func (j *buildJob) SetUpload(ctx context.Context, rc io.ReadCloser) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case fn := <-j.waitCh:
		return fn(rc)
	}
}

// toBuildkitExtraHosts converts hosts from docker key:value format to buildkit's csv format
func toBuildkitExtraHosts(inp []string) (string, error) {
	if len(inp) == 0 {
		return "", nil
	}
	hosts := make([]string, 0, len(inp))
	for _, h := range inp {
		parts := strings.Split(h, ":")
		if len(parts) != 2 || parts[0] == "" || net.ParseIP(parts[1]) == nil {
			return "", errors.Errorf("invalid host %s", h)
		}
		hosts = append(hosts, parts[0]+"="+parts[1])
	}
	return strings.Join(hosts, ","), nil
}
