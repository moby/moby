package filesync

import (
	"os"
	"strings"

	"github.com/moby/buildkit/session"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	keyOverrideExcludes = "override-excludes"
	keyIncludePatterns  = "include-patterns"
)

type fsSyncProvider struct {
	root     string
	excludes []string
	p        progressCb
	doneCh   chan error
}

// NewFSSyncProvider creates a new provider for sending files from client
func NewFSSyncProvider(root string, excludes []string) session.Attachable {
	p := &fsSyncProvider{
		root:     root,
		excludes: excludes,
	}
	return p
}

func (sp *fsSyncProvider) Register(server *grpc.Server) {
	RegisterFileSyncServer(server, sp)
}

func (sp *fsSyncProvider) DiffCopy(stream FileSync_DiffCopyServer) error {
	return sp.handle("diffcopy", stream)
}
func (sp *fsSyncProvider) TarStream(stream FileSync_TarStreamServer) error {
	return sp.handle("tarstream", stream)
}

func (sp *fsSyncProvider) handle(method string, stream grpc.ServerStream) error {
	var pr *protocol
	for _, p := range supportedProtocols {
		if method == p.name && isProtoSupported(p.name) {
			pr = &p
			break
		}
	}
	if pr == nil {
		return errors.New("failed to negotiate protocol")
	}

	opts, _ := metadata.FromContext(stream.Context()) // if no metadata continue with empty object

	var excludes []string
	if len(opts[keyOverrideExcludes]) == 0 || opts[keyOverrideExcludes][0] != "true" {
		excludes = sp.excludes
	}
	includes := opts[keyIncludePatterns]

	var progress progressCb
	if sp.p != nil {
		progress = sp.p
		sp.p = nil
	}

	var doneCh chan error
	if sp.doneCh != nil {
		doneCh = sp.doneCh
		sp.doneCh = nil
	}
	err := pr.sendFn(stream, sp.root, includes, excludes, progress)
	if doneCh != nil {
		if err != nil {
			doneCh <- err
		}
		close(doneCh)
	}
	return err
}

func (sp *fsSyncProvider) SetNextProgressCallback(f func(int, bool), doneCh chan error) {
	sp.p = f
	sp.doneCh = doneCh
}

type progressCb func(int, bool)

type protocol struct {
	name   string
	sendFn func(stream grpc.Stream, srcDir string, includes, excludes []string, progress progressCb) error
	recvFn func(stream grpc.Stream, destDir string, cu CacheUpdater) error
}

func isProtoSupported(p string) bool {
	// TODO: this should be removed after testing if stability is confirmed
	if override := os.Getenv("BUILD_STREAM_PROTOCOL"); override != "" {
		return strings.EqualFold(p, override)
	}
	return true
}

var supportedProtocols = []protocol{
	{
		name:   "diffcopy",
		sendFn: sendDiffCopy,
		recvFn: recvDiffCopy,
	},
	{
		name:   "tarstream",
		sendFn: sendTarStream,
		recvFn: recvTarStream,
	},
}

// FSSendRequestOpt defines options for FSSend request
type FSSendRequestOpt struct {
	IncludePatterns  []string
	OverrideExcludes bool
	DestDir          string
	CacheUpdater     CacheUpdater
}

// CacheUpdater is an object capable of sending notifications for the cache hash changes
type CacheUpdater interface {
	MarkSupported(bool)
	HandleChange(fsutil.ChangeKind, string, os.FileInfo, error) error
}

// FSSync initializes a transfer of files
func FSSync(ctx context.Context, c session.Caller, opt FSSendRequestOpt) error {
	var pr *protocol
	for _, p := range supportedProtocols {
		if isProtoSupported(p.name) && c.Supports(session.MethodURL(_FileSync_serviceDesc.ServiceName, p.name)) {
			pr = &p
			break
		}
	}
	if pr == nil {
		return errors.New("no fssync handlers")
	}

	opts := make(map[string][]string)
	if opt.OverrideExcludes {
		opts[keyOverrideExcludes] = []string{"true"}
	}

	if opt.IncludePatterns != nil {
		opts[keyIncludePatterns] = opt.IncludePatterns
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	client := NewFileSyncClient(c.Conn())

	var stream grpc.ClientStream

	ctx = metadata.NewContext(ctx, opts)

	switch pr.name {
	case "tarstream":
		cc, err := client.TarStream(ctx)
		if err != nil {
			return err
		}
		stream = cc
	case "diffcopy":
		cc, err := client.DiffCopy(ctx)
		if err != nil {
			return err
		}
		stream = cc
	}

	return pr.recvFn(stream, opt.DestDir, opt.CacheUpdater)
}
