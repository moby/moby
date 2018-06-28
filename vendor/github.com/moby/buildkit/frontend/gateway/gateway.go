package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/frontend"
	pb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/tracing"
	"github.com/moby/buildkit/worker"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	keySource           = "source"
	keyDevel            = "gateway-devel"
	exporterImageConfig = "containerimage.config"
)

func NewGatewayFrontend() frontend.Frontend {
	return &gatewayFrontend{}
}

type gatewayFrontend struct {
}

func filterPrefix(opts map[string]string, pfx string) map[string]string {
	m := map[string]string{}
	for k, v := range opts {
		if strings.HasPrefix(k, pfx) {
			m[strings.TrimPrefix(k, pfx)] = v
		}
	}
	return m
}

func (gf *gatewayFrontend) Solve(ctx context.Context, llbBridge frontend.FrontendLLBBridge, opts map[string]string) (retRef solver.CachedResult, exporterAttr map[string][]byte, retErr error) {
	source, ok := opts[keySource]
	if !ok {
		return nil, nil, errors.Errorf("no source specified for gateway")
	}

	sid := session.FromContext(ctx)

	_, isDevel := opts[keyDevel]
	var img specs.Image
	var rootFS cache.ImmutableRef
	var readonly bool // TODO: try to switch to read-only by default.

	if isDevel {
		ref, exp, err := llbBridge.Solve(session.NewContext(ctx, "gateway:"+sid),
			frontend.SolveRequest{
				Frontend:    source,
				FrontendOpt: filterPrefix(opts, "gateway-"),
			})
		if err != nil {
			return nil, nil, err
		}
		defer ref.Release(context.TODO())

		workerRef, ok := ref.Sys().(*worker.WorkerRef)
		if !ok {
			return nil, nil, errors.Errorf("invalid ref: %T", ref.Sys())
		}
		rootFS = workerRef.ImmutableRef
		config, ok := exp[exporterImageConfig]
		if ok {
			if err := json.Unmarshal(config, &img); err != nil {
				return nil, nil, err
			}
		}
	} else {
		sourceRef, err := reference.ParseNormalizedNamed(source)
		if err != nil {
			return nil, nil, err
		}

		dgst, config, err := llbBridge.ResolveImageConfig(ctx, reference.TagNameOnly(sourceRef).String(), nil) // TODO:
		if err != nil {
			return nil, nil, err
		}

		if err := json.Unmarshal(config, &img); err != nil {
			return nil, nil, err
		}

		if dgst != "" {
			sourceRef, err = reference.WithDigest(sourceRef, dgst)
			if err != nil {
				return nil, nil, err
			}
		}

		src := llb.Image(sourceRef.String())

		def, err := src.Marshal()
		if err != nil {
			return nil, nil, err
		}

		ref, _, err := llbBridge.Solve(ctx, frontend.SolveRequest{
			Definition: def.ToPB(),
		})
		if err != nil {
			return nil, nil, err
		}
		defer ref.Release(context.TODO())
		workerRef, ok := ref.Sys().(*worker.WorkerRef)
		if !ok {
			return nil, nil, errors.Errorf("invalid ref: %T", ref.Sys())
		}
		rootFS = workerRef.ImmutableRef
	}

	lbf, err := newLLBBridgeForwarder(ctx, llbBridge)
	defer lbf.conn.Close()
	if err != nil {
		return nil, nil, err
	}

	args := []string{"/run"}
	env := []string{}
	cwd := "/"
	if img.Config.Env != nil {
		env = img.Config.Env
	}
	if img.Config.Entrypoint != nil {
		args = img.Config.Entrypoint
	}
	if img.Config.WorkingDir != "" {
		cwd = img.Config.WorkingDir
	}
	i := 0
	for k, v := range opts {
		env = append(env, fmt.Sprintf("BUILDKIT_FRONTEND_OPT_%d", i)+"="+k+"="+v)
		i++
	}

	env = append(env, "BUILDKIT_SESSION_ID="+sid)

	defer func() {
		for _, r := range lbf.refs {
			if r != nil && (lbf.lastRef != r || retErr != nil) {
				r.Release(context.TODO())
			}
		}
	}()

	err = llbBridge.Exec(ctx, executor.Meta{
		Env:            env,
		Args:           args,
		Cwd:            cwd,
		ReadonlyRootFS: readonly,
	}, rootFS, lbf.Stdin, lbf.Stdout, os.Stderr)

	if err != nil {
		return nil, nil, err
	}

	return lbf.lastRef, lbf.exporterAttr, nil
}

func newLLBBridgeForwarder(ctx context.Context, llbBridge frontend.FrontendLLBBridge) (*llbBridgeForwarder, error) {
	lbf := &llbBridgeForwarder{
		callCtx:   ctx,
		llbBridge: llbBridge,
		refs:      map[string]solver.Result{},
		pipe:      newPipe(),
	}

	server := grpc.NewServer()
	grpc_health_v1.RegisterHealthServer(server, health.NewServer())
	pb.RegisterLLBBridgeServer(server, lbf)

	go serve(ctx, server, lbf.conn)

	return lbf, nil
}

type pipe struct {
	Stdin  io.ReadCloser
	Stdout io.WriteCloser
	conn   net.Conn
}

func newPipe() *pipe {
	pr1, pw1, _ := os.Pipe()
	pr2, pw2, _ := os.Pipe()
	return &pipe{
		Stdin:  pr1,
		Stdout: pw2,
		conn: &conn{
			Reader: pr2,
			Writer: pw1,
			Closer: pw2,
		},
	}
}

type conn struct {
	io.Reader
	io.Writer
	io.Closer
}

func (s *conn) LocalAddr() net.Addr {
	return dummyAddr{}
}
func (s *conn) RemoteAddr() net.Addr {
	return dummyAddr{}
}
func (s *conn) SetDeadline(t time.Time) error {
	return nil
}
func (s *conn) SetReadDeadline(t time.Time) error {
	return nil
}
func (s *conn) SetWriteDeadline(t time.Time) error {
	return nil
}

type dummyAddr struct {
}

func (d dummyAddr) Network() string {
	return "pipe"
}

func (d dummyAddr) String() string {
	return "localhost"
}

type llbBridgeForwarder struct {
	mu           sync.Mutex
	callCtx      context.Context
	llbBridge    frontend.FrontendLLBBridge
	refs         map[string]solver.Result
	lastRef      solver.CachedResult
	exporterAttr map[string][]byte
	*pipe
}

func (lbf *llbBridgeForwarder) ResolveImageConfig(ctx context.Context, req *pb.ResolveImageConfigRequest) (*pb.ResolveImageConfigResponse, error) {
	ctx = tracing.ContextWithSpanFromContext(ctx, lbf.callCtx)
	var platform *specs.Platform
	if p := req.Platform; p != nil {
		platform = &specs.Platform{
			OS:           p.OS,
			Architecture: p.Architecture,
			Variant:      p.Variant,
			OSVersion:    p.OSVersion,
			OSFeatures:   p.OSFeatures,
		}
	}
	dgst, dt, err := lbf.llbBridge.ResolveImageConfig(ctx, req.Ref, platform)
	if err != nil {
		return nil, err
	}
	return &pb.ResolveImageConfigResponse{
		Digest: dgst,
		Config: dt,
	}, nil
}

func (lbf *llbBridgeForwarder) Solve(ctx context.Context, req *pb.SolveRequest) (*pb.SolveResponse, error) {
	ctx = tracing.ContextWithSpanFromContext(ctx, lbf.callCtx)
	ref, expResp, err := lbf.llbBridge.Solve(ctx, frontend.SolveRequest{
		Definition:      req.Definition,
		Frontend:        req.Frontend,
		FrontendOpt:     req.FrontendOpt,
		ImportCacheRefs: req.ImportCacheRefs,
	})
	if err != nil {
		return nil, err
	}

	exp := map[string][]byte{}
	if err := json.Unmarshal(req.ExporterAttr, &exp); err != nil {
		return nil, err
	}

	if expResp != nil {
		for k, v := range expResp {
			exp[k] = v
		}
	}

	id := identity.NewID()
	lbf.mu.Lock()
	lbf.refs[id] = ref
	lbf.mu.Unlock()
	if req.Final {
		lbf.lastRef = ref
		lbf.exporterAttr = exp
	}
	if ref == nil {
		id = ""
	}
	return &pb.SolveResponse{Ref: id}, nil
}
func (lbf *llbBridgeForwarder) ReadFile(ctx context.Context, req *pb.ReadFileRequest) (*pb.ReadFileResponse, error) {
	ctx = tracing.ContextWithSpanFromContext(ctx, lbf.callCtx)
	lbf.mu.Lock()
	ref, ok := lbf.refs[req.Ref]
	lbf.mu.Unlock()
	if !ok {
		return nil, errors.Errorf("no such ref: %v", req.Ref)
	}
	if ref == nil {
		return nil, errors.Wrapf(os.ErrNotExist, "%s no found", req.FilePath)
	}
	workerRef, ok := ref.Sys().(*worker.WorkerRef)
	if !ok {
		return nil, errors.Errorf("invalid ref: %T", ref.Sys())
	}

	newReq := cache.ReadRequest{
		Filename: req.FilePath,
	}
	if r := req.Range; r != nil {
		newReq.Range = &cache.FileRange{
			Offset: int(r.Offset),
			Length: int(r.Length),
		}
	}

	dt, err := cache.ReadFile(ctx, workerRef.ImmutableRef, newReq)
	if err != nil {
		return nil, err
	}

	return &pb.ReadFileResponse{Data: dt}, nil
}

func (lbf *llbBridgeForwarder) Ping(context.Context, *pb.PingRequest) (*pb.PongResponse, error) {
	return &pb.PongResponse{}, nil
}

func serve(ctx context.Context, grpcServer *grpc.Server, conn net.Conn) {
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	logrus.Debugf("serving grpc connection")
	(&http2.Server{}).ServeConn(conn, &http2.ServeConnOpts{Handler: grpcServer})
}
