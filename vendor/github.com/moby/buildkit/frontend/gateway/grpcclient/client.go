package grpcclient

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

	"github.com/gogo/googleapis/google/rpc"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/frontend/gateway/errdefs"
	pb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/identity"
	opspb "github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/apicaps"
	"github.com/moby/buildkit/util/grpcerrors"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	fstypes "github.com/tonistiigi/fsutil/types"
	"golang.org/x/sync/errgroup"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const frontendPrefix = "BUILDKIT_FRONTEND_OPT_"

type GrpcClient interface {
	client.Client
	Run(context.Context, client.BuildFunc) error
}

func New(ctx context.Context, opts map[string]string, session, product string, c pb.LLBBridgeClient, w []client.WorkerInfo) (GrpcClient, error) {
	pingCtx, pingCancel := context.WithTimeout(ctx, 15*time.Second)
	defer pingCancel()
	resp, err := c.Ping(pingCtx, &pb.PingRequest{})
	if err != nil {
		return nil, err
	}

	if resp.FrontendAPICaps == nil {
		resp.FrontendAPICaps = defaultCaps()
	}

	if resp.LLBCaps == nil {
		resp.LLBCaps = defaultLLBCaps()
	}

	return &grpcClient{
		client:    c,
		opts:      opts,
		sessionID: session,
		workers:   w,
		product:   product,
		caps:      pb.Caps.CapSet(resp.FrontendAPICaps),
		llbCaps:   opspb.Caps.CapSet(resp.LLBCaps),
		requests:  map[string]*pb.SolveRequest{},
		execMsgs:  newMessageForwarder(ctx, c),
	}, nil
}

func current() (GrpcClient, error) {
	if ep := product(); ep != "" {
		apicaps.ExportedProduct = ep
	}

	ctx, conn, err := grpcClientConn(context.Background())
	if err != nil {
		return nil, err
	}

	return New(ctx, opts(), sessionID(), product(), pb.NewLLBBridgeClient(conn), workers())
}

func convertRef(ref client.Reference) (*pb.Ref, error) {
	if ref == nil {
		return &pb.Ref{}, nil
	}
	r, ok := ref.(*reference)
	if !ok {
		return nil, errors.Errorf("invalid return reference type %T", ref)
	}
	return &pb.Ref{Id: r.id, Def: r.def}, nil
}

func RunFromEnvironment(ctx context.Context, f client.BuildFunc) error {
	client, err := current()
	if err != nil {
		return errors.Wrapf(err, "failed to initialize client from environment")
	}
	return client.Run(ctx, f)
}

func (c *grpcClient) Run(ctx context.Context, f client.BuildFunc) (retError error) {
	export := c.caps.Supports(pb.CapReturnResult) == nil

	var (
		res *client.Result
		err error
	)
	if export {
		defer func() {
			req := &pb.ReturnRequest{}
			if retError == nil {
				if res == nil {
					res = &client.Result{}
				}
				pbRes := &pb.Result{
					Metadata: res.Metadata,
				}
				if res.Refs != nil {
					if c.caps.Supports(pb.CapProtoRefArray) == nil {
						m := map[string]*pb.Ref{}
						for k, r := range res.Refs {
							pbRef, err := convertRef(r)
							if err != nil {
								retError = err
								continue
							}
							m[k] = pbRef
						}
						pbRes.Result = &pb.Result_Refs{Refs: &pb.RefMap{Refs: m}}
					} else {
						// Server doesn't support the new wire format for refs, so we construct
						// a deprecated result ref map.
						m := map[string]string{}
						for k, r := range res.Refs {
							pbRef, err := convertRef(r)
							if err != nil {
								retError = err
								continue
							}
							m[k] = pbRef.Id
						}
						pbRes.Result = &pb.Result_RefsDeprecated{RefsDeprecated: &pb.RefMapDeprecated{Refs: m}}
					}
				} else {
					pbRef, err := convertRef(res.Ref)
					if err != nil {
						retError = err
					} else {
						if c.caps.Supports(pb.CapProtoRefArray) == nil {
							pbRes.Result = &pb.Result_Ref{Ref: pbRef}
						} else {
							// Server doesn't support the new wire format for refs, so we construct
							// a deprecated result ref.
							pbRes.Result = &pb.Result_RefDeprecated{RefDeprecated: pbRef.Id}
						}
					}
				}
				if retError == nil {
					req.Result = pbRes
				}
			}
			if retError != nil {
				st, _ := status.FromError(grpcerrors.ToGRPC(retError))
				stp := st.Proto()
				req.Error = &rpc.Status{
					Code:    stp.Code,
					Message: stp.Message,
					Details: convertToGogoAny(stp.Details),
				}
			}
			if _, err := c.client.Return(ctx, req); err != nil && retError == nil {
				retError = err
			}
		}()
	}

	defer func() {
		err = c.execMsgs.Release()
		if err != nil && retError != nil {
			retError = err
		}
	}()

	if res, err = f(ctx, c); err != nil {
		return err
	}

	if res == nil {
		return nil
	}

	if err := c.caps.Supports(pb.CapReturnMap); len(res.Refs) > 1 && err != nil {
		return err
	}

	if !export {
		exportedAttrBytes, err := json.Marshal(res.Metadata)
		if err != nil {
			return errors.Wrapf(err, "failed to marshal return metadata")
		}

		req, err := c.requestForRef(res.Ref)
		if err != nil {
			return errors.Wrapf(err, "failed to find return ref")
		}

		req.Final = true
		req.ExporterAttr = exportedAttrBytes

		if _, err := c.client.Solve(ctx, req); err != nil {
			return errors.Wrapf(err, "failed to solve")
		}
	}

	return nil
}

// defaultCaps returns the capabilities that were implemented when capabilities
// support was added. This list is frozen and should never be changed.
func defaultCaps() []apicaps.PBCap {
	return []apicaps.PBCap{
		{ID: string(pb.CapSolveBase), Enabled: true},
		{ID: string(pb.CapSolveInlineReturn), Enabled: true},
		{ID: string(pb.CapResolveImage), Enabled: true},
		{ID: string(pb.CapReadFile), Enabled: true},
	}
}

// defaultLLBCaps returns the LLB capabilities that were implemented when capabilities
// support was added. This list is frozen and should never be changed.
func defaultLLBCaps() []apicaps.PBCap {
	return []apicaps.PBCap{
		{ID: string(opspb.CapSourceImage), Enabled: true},
		{ID: string(opspb.CapSourceLocal), Enabled: true},
		{ID: string(opspb.CapSourceLocalUnique), Enabled: true},
		{ID: string(opspb.CapSourceLocalSessionID), Enabled: true},
		{ID: string(opspb.CapSourceLocalIncludePatterns), Enabled: true},
		{ID: string(opspb.CapSourceLocalFollowPaths), Enabled: true},
		{ID: string(opspb.CapSourceLocalExcludePatterns), Enabled: true},
		{ID: string(opspb.CapSourceLocalSharedKeyHint), Enabled: true},
		{ID: string(opspb.CapSourceGit), Enabled: true},
		{ID: string(opspb.CapSourceGitKeepDir), Enabled: true},
		{ID: string(opspb.CapSourceGitFullURL), Enabled: true},
		{ID: string(opspb.CapSourceHTTP), Enabled: true},
		{ID: string(opspb.CapSourceHTTPChecksum), Enabled: true},
		{ID: string(opspb.CapSourceHTTPPerm), Enabled: true},
		{ID: string(opspb.CapSourceHTTPUIDGID), Enabled: true},
		{ID: string(opspb.CapBuildOpLLBFileName), Enabled: true},
		{ID: string(opspb.CapExecMetaBase), Enabled: true},
		{ID: string(opspb.CapExecMetaProxy), Enabled: true},
		{ID: string(opspb.CapExecMountBind), Enabled: true},
		{ID: string(opspb.CapExecMountCache), Enabled: true},
		{ID: string(opspb.CapExecMountCacheSharing), Enabled: true},
		{ID: string(opspb.CapExecMountSelector), Enabled: true},
		{ID: string(opspb.CapExecMountTmpfs), Enabled: true},
		{ID: string(opspb.CapExecMountSecret), Enabled: true},
		{ID: string(opspb.CapConstraints), Enabled: true},
		{ID: string(opspb.CapPlatform), Enabled: true},
		{ID: string(opspb.CapMetaIgnoreCache), Enabled: true},
		{ID: string(opspb.CapMetaDescription), Enabled: true},
		{ID: string(opspb.CapMetaExportCache), Enabled: true},
	}
}

type grpcClient struct {
	client    pb.LLBBridgeClient
	opts      map[string]string
	sessionID string
	product   string
	workers   []client.WorkerInfo
	caps      apicaps.CapSet
	llbCaps   apicaps.CapSet
	requests  map[string]*pb.SolveRequest
	execMsgs  *messageForwarder
}

func (c *grpcClient) requestForRef(ref client.Reference) (*pb.SolveRequest, error) {
	emptyReq := &pb.SolveRequest{
		Definition: &opspb.Definition{},
	}
	if ref == nil {
		return emptyReq, nil
	}
	r, ok := ref.(*reference)
	if !ok {
		return nil, errors.Errorf("return reference has invalid type %T", ref)
	}
	if r.id == "" {
		return emptyReq, nil
	}
	req, ok := c.requests[r.id]
	if !ok {
		return nil, errors.Errorf("did not find request for return reference %s", r.id)
	}
	return req, nil
}

func (c *grpcClient) Solve(ctx context.Context, creq client.SolveRequest) (res *client.Result, err error) {
	if creq.Definition != nil {
		for _, md := range creq.Definition.Metadata {
			for cap := range md.Caps {
				if err := c.llbCaps.Supports(cap); err != nil {
					return nil, err
				}
			}
		}
	}
	var (
		// old API
		legacyRegistryCacheImports []string
		// new API (CapImportCaches)
		cacheImports []*pb.CacheOptionsEntry
	)
	supportCapImportCaches := c.caps.Supports(pb.CapImportCaches) == nil
	for _, im := range creq.CacheImports {
		if !supportCapImportCaches && im.Type == "registry" {
			legacyRegistryCacheImports = append(legacyRegistryCacheImports, im.Attrs["ref"])
		} else {
			cacheImports = append(cacheImports, &pb.CacheOptionsEntry{
				Type:  im.Type,
				Attrs: im.Attrs,
			})
		}
	}

	req := &pb.SolveRequest{
		Definition:          creq.Definition,
		Frontend:            creq.Frontend,
		FrontendOpt:         creq.FrontendOpt,
		FrontendInputs:      creq.FrontendInputs,
		AllowResultReturn:   true,
		AllowResultArrayRef: true,
		// old API
		ImportCacheRefsDeprecated: legacyRegistryCacheImports,
		// new API
		CacheImports: cacheImports,
	}

	// backwards compatibility with inline return
	if c.caps.Supports(pb.CapReturnResult) != nil {
		req.ExporterAttr = []byte("{}")
	}

	if creq.Evaluate {
		if c.caps.Supports(pb.CapGatewayEvaluateSolve) == nil {
			req.Evaluate = creq.Evaluate
		} else {
			// If evaluate is not supported, fallback to running Stat(".") in order to
			// trigger an evaluation of the result.
			defer func() {
				if res == nil {
					return
				}

				var (
					id  string
					ref client.Reference
				)
				ref, err = res.SingleRef()
				if err != nil {
					for refID := range res.Refs {
						id = refID
						break
					}
				} else {
					id = ref.(*reference).id
				}

				_, err = c.client.StatFile(ctx, &pb.StatFileRequest{
					Ref:  id,
					Path: ".",
				})
			}()
		}
	}

	resp, err := c.client.Solve(ctx, req)
	if err != nil {
		return nil, err
	}

	res = &client.Result{}
	if resp.Result == nil {
		if id := resp.Ref; id != "" {
			c.requests[id] = req
		}
		res.SetRef(&reference{id: resp.Ref, c: c})
	} else {
		res.Metadata = resp.Result.Metadata
		switch pbRes := resp.Result.Result.(type) {
		case *pb.Result_RefDeprecated:
			if id := pbRes.RefDeprecated; id != "" {
				res.SetRef(&reference{id: id, c: c})
			}
		case *pb.Result_RefsDeprecated:
			for k, v := range pbRes.RefsDeprecated.Refs {
				ref := &reference{id: v, c: c}
				if v == "" {
					ref = nil
				}
				res.AddRef(k, ref)
			}
		case *pb.Result_Ref:
			if pbRes.Ref.Id != "" {
				ref, err := newReference(c, pbRes.Ref)
				if err != nil {
					return nil, err
				}
				res.SetRef(ref)
			}
		case *pb.Result_Refs:
			for k, v := range pbRes.Refs.Refs {
				var ref *reference
				if v.Id != "" {
					ref, err = newReference(c, v)
					if err != nil {
						return nil, err
					}
				}
				res.AddRef(k, ref)
			}
		}
	}

	return res, nil
}

func (c *grpcClient) ResolveImageConfig(ctx context.Context, ref string, opt llb.ResolveImageConfigOpt) (digest.Digest, []byte, error) {
	var p *opspb.Platform
	if platform := opt.Platform; platform != nil {
		p = &opspb.Platform{
			OS:           platform.OS,
			Architecture: platform.Architecture,
			Variant:      platform.Variant,
			OSVersion:    platform.OSVersion,
			OSFeatures:   platform.OSFeatures,
		}
	}
	resp, err := c.client.ResolveImageConfig(ctx, &pb.ResolveImageConfigRequest{Ref: ref, Platform: p, ResolveMode: opt.ResolveMode, LogName: opt.LogName})
	if err != nil {
		return "", nil, err
	}
	return resp.Digest, resp.Config, nil
}

func (c *grpcClient) BuildOpts() client.BuildOpts {
	return client.BuildOpts{
		Opts:      c.opts,
		SessionID: c.sessionID,
		Workers:   c.workers,
		Product:   c.product,
		LLBCaps:   c.llbCaps,
		Caps:      c.caps,
	}
}

func (c *grpcClient) Inputs(ctx context.Context) (map[string]llb.State, error) {
	err := c.caps.Supports(pb.CapFrontendInputs)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Inputs(ctx, &pb.InputsRequest{})
	if err != nil {
		return nil, err
	}

	inputs := make(map[string]llb.State)
	for key, def := range resp.Definitions {
		op, err := llb.NewDefinitionOp(def)
		if err != nil {
			return nil, err
		}
		inputs[key] = llb.NewState(op)
	}
	return inputs, nil
}

// procMessageForwarder is created per container process to act as the
// communication channel between the process and the ExecProcess message
// stream.
type procMessageForwarder struct {
	done      chan struct{}
	closeOnce sync.Once
	msgs      chan *pb.ExecMessage
}

func newProcMessageForwarder() *procMessageForwarder {
	return &procMessageForwarder{
		done: make(chan struct{}),
		msgs: make(chan *pb.ExecMessage),
	}
}

func (b *procMessageForwarder) Send(ctx context.Context, m *pb.ExecMessage) {
	select {
	case <-ctx.Done():
	case <-b.done:
		b.closeOnce.Do(func() {
			close(b.msgs)
		})
	case b.msgs <- m:
	}
}

func (b *procMessageForwarder) Recv(ctx context.Context) (m *pb.ExecMessage, ok bool) {
	select {
	case <-ctx.Done():
		return nil, true
	case <-b.done:
		return nil, false
	case m = <-b.msgs:
		return m, true
	}
}

func (b *procMessageForwarder) Close() {
	close(b.done)
	b.Recv(context.Background())      // flush any messages in queue
	b.Send(context.Background(), nil) // ensure channel is closed
}

// messageForwarder manages a single grpc stream for ExecProcess to facilitate
// a pub/sub message channel for each new process started from the client
// connection.
type messageForwarder struct {
	client pb.LLBBridgeClient
	ctx    context.Context
	cancel func()
	eg     *errgroup.Group
	mu     sync.Mutex
	pids   map[string]*procMessageForwarder
	stream pb.LLBBridge_ExecProcessClient
	// startOnce used to only start the exec message forwarder once,
	// so we only have one exec stream per client
	startOnce sync.Once
	// startErr tracks the error when initializing the stream, it will
	// be returned on subsequent calls to Start
	startErr error
}

func newMessageForwarder(ctx context.Context, client pb.LLBBridgeClient) *messageForwarder {
	ctx, cancel := context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)
	return &messageForwarder{
		client: client,
		pids:   map[string]*procMessageForwarder{},
		ctx:    ctx,
		cancel: cancel,
		eg:     eg,
	}
}

func (m *messageForwarder) Start() (err error) {
	defer func() {
		if err != nil {
			m.startErr = err
		}
	}()

	if m.startErr != nil {
		return m.startErr
	}

	m.startOnce.Do(func() {
		m.stream, err = m.client.ExecProcess(m.ctx)
		if err != nil {
			return
		}
		m.eg.Go(func() error {
			for {
				msg, err := m.stream.Recv()
				if errors.Is(err, io.EOF) || grpcerrors.Code(err) == codes.Canceled {
					return nil
				}
				logrus.Debugf("|<--- %s", debugMessage(msg))

				if err != nil {
					return err
				}

				m.mu.Lock()
				msgs, ok := m.pids[msg.ProcessID]
				m.mu.Unlock()

				if !ok {
					logrus.Debugf("Received exec message for unregistered process: %s", msg.String())
					continue
				}
				msgs.Send(m.ctx, msg)
			}
		})
	})
	return err
}

func debugMessage(msg *pb.ExecMessage) string {
	switch m := msg.GetInput().(type) {
	case *pb.ExecMessage_Init:
		return fmt.Sprintf("Init Message %s", msg.ProcessID)
	case *pb.ExecMessage_File:
		if m.File.EOF {
			return fmt.Sprintf("File Message %s, fd=%d, EOF", msg.ProcessID, m.File.Fd)
		}
		return fmt.Sprintf("File Message %s, fd=%d, %d bytes", msg.ProcessID, m.File.Fd, len(m.File.Data))
	case *pb.ExecMessage_Resize:
		return fmt.Sprintf("Resize Message %s", msg.ProcessID)
	case *pb.ExecMessage_Started:
		return fmt.Sprintf("Started Message %s", msg.ProcessID)
	case *pb.ExecMessage_Exit:
		return fmt.Sprintf("Exit Message %s, code=%d, err=%s", msg.ProcessID, m.Exit.Code, m.Exit.Error)
	case *pb.ExecMessage_Done:
		return fmt.Sprintf("Done Message %s", msg.ProcessID)
	}
	return fmt.Sprintf("Unknown Message %s", msg.String())
}

func (m *messageForwarder) Send(msg *pb.ExecMessage) error {
	m.mu.Lock()
	_, ok := m.pids[msg.ProcessID]
	defer m.mu.Unlock()
	if !ok {
		return errors.Errorf("process %s has ended, not sending message %#v", msg.ProcessID, msg.Input)
	}
	logrus.Debugf("|---> %s", debugMessage(msg))
	return m.stream.Send(msg)
}

func (m *messageForwarder) Release() error {
	m.cancel()
	return m.eg.Wait()
}

func (m *messageForwarder) Register(pid string) *procMessageForwarder {
	m.mu.Lock()
	defer m.mu.Unlock()
	sender := newProcMessageForwarder()
	m.pids[pid] = sender
	return sender
}

func (m *messageForwarder) Deregister(pid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sender, ok := m.pids[pid]
	if !ok {
		return
	}
	delete(m.pids, pid)
	sender.Close()
}

type msgWriter struct {
	mux       *messageForwarder
	fd        uint32
	processID string
}

func (w *msgWriter) Write(msg []byte) (int, error) {
	err := w.mux.Send(&pb.ExecMessage{
		ProcessID: w.processID,
		Input: &pb.ExecMessage_File{
			File: &pb.FdMessage{
				Fd:   w.fd,
				Data: msg,
			},
		},
	})
	if err != nil {
		return 0, err
	}
	return len(msg), nil
}

func (c *grpcClient) NewContainer(ctx context.Context, req client.NewContainerRequest) (client.Container, error) {
	err := c.caps.Supports(pb.CapGatewayExec)
	if err != nil {
		return nil, err
	}
	id := identity.NewID()
	var mounts []*opspb.Mount
	for _, m := range req.Mounts {
		resultID := m.ResultID
		if m.Ref != nil {
			ref, ok := m.Ref.(*reference)
			if !ok {
				return nil, errors.Errorf("unexpected type for reference, got %T", m.Ref)
			}
			resultID = ref.id
		}
		mounts = append(mounts, &opspb.Mount{
			Dest:      m.Dest,
			Selector:  m.Selector,
			Readonly:  m.Readonly,
			MountType: m.MountType,
			ResultID:  resultID,
			CacheOpt:  m.CacheOpt,
			SecretOpt: m.SecretOpt,
			SSHOpt:    m.SSHOpt,
		})
	}

	logrus.Debugf("|---> NewContainer %s", id)
	_, err = c.client.NewContainer(ctx, &pb.NewContainerRequest{
		ContainerID: id,
		Mounts:      mounts,
		Platform:    req.Platform,
		Constraints: req.Constraints,
	})
	if err != nil {
		return nil, err
	}

	// ensure message forwarder is started, only sets up stream first time called
	err = c.execMsgs.Start()
	if err != nil {
		return nil, err
	}

	return &container{
		client:   c.client,
		id:       id,
		execMsgs: c.execMsgs,
	}, nil
}

type container struct {
	client   pb.LLBBridgeClient
	id       string
	execMsgs *messageForwarder
}

func (ctr *container) Start(ctx context.Context, req client.StartRequest) (client.ContainerProcess, error) {
	pid := fmt.Sprintf("%s:%s", ctr.id, identity.NewID())
	msgs := ctr.execMsgs.Register(pid)

	init := &pb.InitMessage{
		ContainerID: ctr.id,
		Meta: &opspb.Meta{
			Args: req.Args,
			Env:  req.Env,
			Cwd:  req.Cwd,
			User: req.User,
		},
		Tty:      req.Tty,
		Security: req.SecurityMode,
	}
	if req.Stdin != nil {
		init.Fds = append(init.Fds, 0)
	}
	if req.Stdout != nil {
		init.Fds = append(init.Fds, 1)
	}
	if req.Stderr != nil {
		init.Fds = append(init.Fds, 2)
	}

	err := ctr.execMsgs.Send(&pb.ExecMessage{
		ProcessID: pid,
		Input: &pb.ExecMessage_Init{
			Init: init,
		},
	})
	if err != nil {
		return nil, err
	}

	msg, _ := msgs.Recv(ctx)
	if msg == nil {
		return nil, errors.Errorf("failed to receive started message")
	}
	started := msg.GetStarted()
	if started == nil {
		return nil, errors.Errorf("expecting started message, got %T", msg.GetInput())
	}

	eg, ctx := errgroup.WithContext(ctx)
	done := make(chan struct{})

	ctrProc := &containerProcess{
		execMsgs: ctr.execMsgs,
		id:       pid,
		eg:       eg,
	}

	var stdinReader *io.PipeReader
	ctrProc.eg.Go(func() error {
		<-done
		if stdinReader != nil {
			return stdinReader.Close()
		}
		return nil
	})

	if req.Stdin != nil {
		var stdinWriter io.WriteCloser
		stdinReader, stdinWriter = io.Pipe()
		// This go routine is intentionally not part of the errgroup because
		// if os.Stdin is used for req.Stdin then this will block until
		// the user closes the input, which will likely be after we are done
		// with the container, so we can't Wait on it.
		go func() {
			io.Copy(stdinWriter, req.Stdin)
			stdinWriter.Close()
		}()

		ctrProc.eg.Go(func() error {
			m := &msgWriter{
				mux:       ctr.execMsgs,
				processID: pid,
				fd:        0,
			}
			_, err := io.Copy(m, stdinReader)
			// ignore ErrClosedPipe, it is EOF for our usage.
			if err != nil && !errors.Is(err, io.ErrClosedPipe) {
				return err
			}
			// not an error so must be eof
			return ctr.execMsgs.Send(&pb.ExecMessage{
				ProcessID: pid,
				Input: &pb.ExecMessage_File{
					File: &pb.FdMessage{
						Fd:  0,
						EOF: true,
					},
				},
			})
		})
	}

	ctrProc.eg.Go(func() error {
		var closeDoneOnce sync.Once
		var exitError error
		for {
			msg, ok := msgs.Recv(ctx)
			if !ok {
				// no more messages, return
				return exitError
			}

			if msg == nil {
				// empty message from ctx cancel, so just start shutting down
				// input, but continue processing more exit/done messages
				closeDoneOnce.Do(func() {
					close(done)
				})
				continue
			}

			if file := msg.GetFile(); file != nil {
				var out io.WriteCloser
				switch file.Fd {
				case 1:
					out = req.Stdout
				case 2:
					out = req.Stderr
				}
				if out == nil {
					// if things are plumbed correctly this should never happen
					return errors.Errorf("missing writer for output fd %d", file.Fd)
				}
				if len(file.Data) > 0 {
					_, err := out.Write(file.Data)
					if err != nil {
						return err
					}
				}
			} else if exit := msg.GetExit(); exit != nil {
				// capture exit message to exitError so we can return it after
				// the server sends the Done message
				closeDoneOnce.Do(func() {
					close(done)
				})
				if exit.Code == 0 {
					continue
				}
				exitError = grpcerrors.FromGRPC(status.ErrorProto(&spb.Status{
					Code:    exit.Error.Code,
					Message: exit.Error.Message,
					Details: convertGogoAny(exit.Error.Details),
				}))
				if exit.Code != errdefs.UnknownExitStatus {
					exitError = &errdefs.ExitError{ExitCode: exit.Code, Err: exitError}
				}
			} else if serverDone := msg.GetDone(); serverDone != nil {
				return exitError
			} else {
				return errors.Errorf("unexpected Exec Message for pid %s: %T", pid, msg.GetInput())
			}
		}
	})

	return ctrProc, nil
}

func (ctr *container) Release(ctx context.Context) error {
	logrus.Debugf("|---> ReleaseContainer %s", ctr.id)
	_, err := ctr.client.ReleaseContainer(ctx, &pb.ReleaseContainerRequest{
		ContainerID: ctr.id,
	})
	return err
}

type containerProcess struct {
	execMsgs *messageForwarder
	id       string
	eg       *errgroup.Group
}

func (ctrProc *containerProcess) Wait() error {
	defer ctrProc.execMsgs.Deregister(ctrProc.id)
	return ctrProc.eg.Wait()
}

func (ctrProc *containerProcess) Resize(_ context.Context, size client.WinSize) error {
	return ctrProc.execMsgs.Send(&pb.ExecMessage{
		ProcessID: ctrProc.id,
		Input: &pb.ExecMessage_Resize{
			Resize: &pb.ResizeMessage{
				Cols: size.Cols,
				Rows: size.Rows,
			},
		},
	})
}

type reference struct {
	c   *grpcClient
	id  string
	def *opspb.Definition
}

func newReference(c *grpcClient, ref *pb.Ref) (*reference, error) {
	return &reference{c: c, id: ref.Id, def: ref.Def}, nil
}

func (r *reference) ToState() (st llb.State, err error) {
	err = r.c.caps.Supports(pb.CapReferenceOutput)
	if err != nil {
		return st, err
	}

	if r.def == nil {
		return st, errors.Errorf("gateway did not return reference with definition")
	}

	defop, err := llb.NewDefinitionOp(r.def)
	if err != nil {
		return st, err
	}

	return llb.NewState(defop), nil
}

func (r *reference) ReadFile(ctx context.Context, req client.ReadRequest) ([]byte, error) {
	rfr := &pb.ReadFileRequest{FilePath: req.Filename, Ref: r.id}
	if r := req.Range; r != nil {
		rfr.Range = &pb.FileRange{
			Offset: int64(r.Offset),
			Length: int64(r.Length),
		}
	}
	resp, err := r.c.client.ReadFile(ctx, rfr)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (r *reference) ReadDir(ctx context.Context, req client.ReadDirRequest) ([]*fstypes.Stat, error) {
	if err := r.c.caps.Supports(pb.CapReadDir); err != nil {
		return nil, err
	}
	rdr := &pb.ReadDirRequest{
		DirPath:        req.Path,
		IncludePattern: req.IncludePattern,
		Ref:            r.id,
	}
	resp, err := r.c.client.ReadDir(ctx, rdr)
	if err != nil {
		return nil, err
	}
	return resp.Entries, nil
}

func (r *reference) StatFile(ctx context.Context, req client.StatRequest) (*fstypes.Stat, error) {
	if err := r.c.caps.Supports(pb.CapStatFile); err != nil {
		return nil, err
	}
	rdr := &pb.StatFileRequest{
		Path: req.Path,
		Ref:  r.id,
	}
	resp, err := r.c.client.StatFile(ctx, rdr)
	if err != nil {
		return nil, err
	}
	return resp.Stat, nil
}

func grpcClientConn(ctx context.Context) (context.Context, *grpc.ClientConn, error) {
	dialOpt := grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		return stdioConn(), nil
	})

	cc, err := grpc.DialContext(ctx, "localhost", dialOpt, grpc.WithInsecure(), grpc.WithUnaryInterceptor(grpcerrors.UnaryClientInterceptor), grpc.WithStreamInterceptor(grpcerrors.StreamClientInterceptor))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create grpc client")
	}

	ctx, cancel := context.WithCancel(ctx)
	_ = cancel
	// go monitorHealth(ctx, cc, cancel)

	return ctx, cc, nil
}

func stdioConn() net.Conn {
	return &conn{os.Stdin, os.Stdout, os.Stdout}
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

func opts() map[string]string {
	opts := map[string]string{}
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		k := parts[0]
		v := ""
		if len(parts) == 2 {
			v = parts[1]
		}
		if !strings.HasPrefix(k, frontendPrefix) {
			continue
		}
		parts = strings.SplitN(v, "=", 2)
		v = ""
		if len(parts) == 2 {
			v = parts[1]
		}
		opts[parts[0]] = v
	}
	return opts
}

func sessionID() string {
	return os.Getenv("BUILDKIT_SESSION_ID")
}

func workers() []client.WorkerInfo {
	var c []client.WorkerInfo
	if err := json.Unmarshal([]byte(os.Getenv("BUILDKIT_WORKERS")), &c); err != nil {
		return nil
	}
	return c
}

func product() string {
	return os.Getenv("BUILDKIT_EXPORTEDPRODUCT")
}

func convertGogoAny(in []*gogotypes.Any) []*any.Any {
	out := make([]*any.Any, len(in))
	for i := range in {
		out[i] = &any.Any{TypeUrl: in[i].TypeUrl, Value: in[i].Value}
	}
	return out
}

func convertToGogoAny(in []*any.Any) []*gogotypes.Any {
	out := make([]*gogotypes.Any, len(in))
	for i := range in {
		out[i] = &gogotypes.Any{TypeUrl: in[i].TypeUrl, Value: in[i].Value}
	}
	return out
}
