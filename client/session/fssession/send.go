package fssession

import (
	"os"

	"github.com/docker/docker/client/session"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
	"golang.org/x/net/context"
)

type fsSendProvider struct {
	name     string
	root     string
	excludes []string
	p        progressCb
	doneCh   chan error
}

// NewFSSendProvider creates a new provider for sending files from client
func NewFSSendProvider(name, root string, excludes []string) session.Attachment {
	p := &fsSendProvider{
		name:     name,
		root:     root,
		excludes: excludes,
	}
	return p
}

func (sp *fsSendProvider) RegisterHandlers(fn func(id, method string) error) error {
	for _, p := range supportedProtocols {
		if isProtoSupported(p) {
			if err := fn(sp.name, p.name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (sp *fsSendProvider) Handle(ctx context.Context, id, method string, opts map[string][]string, stream session.Stream) error {
	if id != sp.name {
		return errors.Errorf("invalid id %s", id)
	}

	var pr *protocol
	for _, p := range supportedProtocols {
		if method == p.name && isProtoSupported(p) {
			pr = &p
			break
		}
	}

	if pr == nil {
		return errors.New("failed to negotiate protocol")
	}

	var excludes []string
	if len(opts["Override-Excludes"]) == 0 || opts["Override-Excludes"][0] != "true" {
		excludes = sp.excludes
	}

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
	err := pr.sendFn(stream, sp.root, excludes, progress)
	if doneCh != nil {
		if err != nil {
			doneCh <- err
		}
		close(doneCh)
	}
	return err
}

func (sp *fsSendProvider) SetNextProgressCallback(f func(int, bool), doneCh chan error) {
	sp.p = f
	sp.doneCh = doneCh
}

type progressCb func(int, bool)

type protocol struct {
	name   string
	sendFn func(stream session.Stream, srcDir string, excludes []string, progress progressCb) error
	recvFn func(stream session.Stream, destDir string, cu CacheUpdater) error
}

func isProtoSupported(p protocol) bool {
	if override := os.Getenv("BUILD_STREAM_PROTOCOL"); override != "" {
		return p.name == override
	}
	return true
}

var supportedProtocols = []protocol{
	{
		name:   "tarstream",
		sendFn: sendTarStream,
		recvFn: recvTarStream,
	},
	{
		name:   "diffcopy",
		sendFn: sendDiffCopy,
		recvFn: recvDiffCopy,
	},
}

// FSSendRequestOpt defines options for FSSend request
type FSSendRequestOpt struct {
	SrcPaths         []string
	OverrideExcludes bool
	DestDir          string
	CacheUpdater     CacheUpdater
}

// CacheUpdater is an object capable of sending notifications for the cache hash changes
type CacheUpdater interface {
	MarkSupported(bool)
	HandleChange(fsutil.ChangeKind, string, os.FileInfo, error) error
}

// FSSend initializes a transfer of files
func FSSend(ctx context.Context, name string, c session.Caller, opt FSSendRequestOpt) error {
	var pr *protocol
	for _, p := range supportedProtocols {
		if isProtoSupported(p) && c.Supports(name, p.name) {
			pr = &p
			break
		}
	}
	if pr == nil {
		return errors.Errorf("no fssend handlers for %s", name)
	}

	opts := make(map[string][]string)
	if opt.OverrideExcludes {
		opts["Override-Excludes"] = []string{"true"}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := c.Call(ctx, name, pr.name, opts)
	if err != nil {
		return err
	}

	return pr.recvFn(stream, opt.DestDir, opt.CacheUpdater)
}
