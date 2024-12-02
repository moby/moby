package filesync

import (
	"context"
	"fmt"
	io "io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/bklog"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
	fstypes "github.com/tonistiigi/fsutil/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	keyIncludePatterns    = "include-patterns"
	keyExcludePatterns    = "exclude-patterns"
	keyFollowPaths        = "followpaths"
	keyDirName            = "dir-name"
	keyExporterMetaPrefix = "exporter-md-"

	keyExporterID = "buildkit-attachable-exporter-id"
)

type fsSyncProvider struct {
	dirs   DirSource
	p      progressCb
	doneCh chan error
}

type FileOutputFunc func(map[string]string) (io.WriteCloser, error)

type SyncedDir struct {
	Dir string
	Map func(string, *fstypes.Stat) fsutil.MapResult
}

type DirSource interface {
	LookupDir(string) (fsutil.FS, bool)
}

type StaticDirSource map[string]fsutil.FS

var _ DirSource = StaticDirSource{}

func (dirs StaticDirSource) LookupDir(name string) (fsutil.FS, bool) {
	dir, found := dirs[name]
	return dir, found
}

// NewFSSyncProvider creates a new provider for sending files from client
func NewFSSyncProvider(dirs DirSource) session.Attachable {
	return &fsSyncProvider{
		dirs: dirs,
	}
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

func (sp *fsSyncProvider) handle(method string, stream grpc.ServerStream) (retErr error) {
	var pr *protocol
	for _, p := range supportedProtocols {
		if method == p.name {
			pr = &p
			break
		}
	}
	if pr == nil {
		return InvalidSessionError{errors.New("failed to negotiate protocol")}
	}

	opts, _ := metadata.FromIncomingContext(stream.Context()) // if no metadata continue with empty object
	opts = decodeOpts(opts)

	dirName := ""
	name, ok := opts[keyDirName]
	if ok && len(name) > 0 {
		dirName = name[0]
	}

	excludes := opts[keyExcludePatterns]
	includes := opts[keyIncludePatterns]
	followPaths := opts[keyFollowPaths]

	dir, ok := sp.dirs.LookupDir(dirName)
	if !ok {
		return InvalidSessionError{status.Errorf(codes.NotFound, "no access allowed to dir %q", dirName)}
	}
	dir, err := fsutil.NewFilterFS(dir, &fsutil.FilterOpt{
		ExcludePatterns: excludes,
		IncludePatterns: includes,
		FollowPaths:     followPaths,
	})
	if err != nil {
		return err
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
	err = pr.sendFn(stream, dir, progress)
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
	sendFn func(stream Stream, fs fsutil.FS, progress progressCb) error
	recvFn func(stream grpc.ClientStream, destDir string, cu CacheUpdater, progress progressCb, differ fsutil.DiffType, mapFunc func(string, *fstypes.Stat) bool) error
}

var supportedProtocols = []protocol{
	{
		name:   "diffcopy",
		sendFn: sendDiffCopy,
		recvFn: recvDiffCopy,
	},
}

// FSSendRequestOpt defines options for FSSend request
type FSSendRequestOpt struct {
	Name            string
	IncludePatterns []string
	ExcludePatterns []string
	FollowPaths     []string
	DestDir         string
	CacheUpdater    CacheUpdater
	ProgressCb      func(int, bool)
	Filter          func(string, *fstypes.Stat) bool
	Differ          fsutil.DiffType
}

// CacheUpdater is an object capable of sending notifications for the cache hash changes
type CacheUpdater interface {
	MarkSupported(bool)
	HandleChange(fsutil.ChangeKind, string, os.FileInfo, error) error
	ContentHasher() fsutil.ContentHasher
}

// FSSync initializes a transfer of files
func FSSync(ctx context.Context, c session.Caller, opt FSSendRequestOpt) error {
	var pr *protocol
	for _, p := range supportedProtocols {
		if c.Supports(session.MethodURL(FileSync_ServiceDesc.ServiceName, p.name)) {
			pr = &p
			break
		}
	}
	if pr == nil {
		return errors.New("no local sources enabled")
	}

	opts := make(map[string][]string)

	if opt.IncludePatterns != nil {
		opts[keyIncludePatterns] = opt.IncludePatterns
	}

	if opt.ExcludePatterns != nil {
		opts[keyExcludePatterns] = opt.ExcludePatterns
	}

	if opt.FollowPaths != nil {
		opts[keyFollowPaths] = opt.FollowPaths
	}

	opts[keyDirName] = []string{opt.Name}

	ctx, cancel := context.WithCancelCause(ctx)
	defer func() { cancel(errors.WithStack(context.Canceled)) }()

	client := NewFileSyncClient(c.Conn())

	var stream grpc.ClientStream

	opts = encodeOpts(opts)

	ctx = metadata.NewOutgoingContext(ctx, opts)

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
	default:
		panic(fmt.Sprintf("invalid protocol: %q", pr.name))
	}

	return pr.recvFn(stream, opt.DestDir, opt.CacheUpdater, opt.ProgressCb, opt.Differ, opt.Filter)
}

type FSSyncTarget interface {
	target() *fsSyncTarget
}

type fsSyncTarget struct {
	id     int
	outdir string
	f      FileOutputFunc
}

func (target *fsSyncTarget) target() *fsSyncTarget {
	return target
}

func WithFSSync(id int, f FileOutputFunc) FSSyncTarget {
	return &fsSyncTarget{
		id: id,
		f:  f,
	}
}

func WithFSSyncDir(id int, outdir string) FSSyncTarget {
	return &fsSyncTarget{
		id:     id,
		outdir: outdir,
	}
}

func NewFSSyncTarget(targets ...FSSyncTarget) session.Attachable {
	fs := make(map[int]FileOutputFunc)
	outdirs := make(map[int]string)
	for _, t := range targets {
		t := t.target()
		if t.f != nil {
			fs[t.id] = t.f
		}
		if t.outdir != "" {
			outdirs[t.id] = t.outdir
		}
	}
	return &fsSyncAttachable{
		fs:      fs,
		outdirs: outdirs,
	}
}

type fsSyncAttachable struct {
	fs      map[int]FileOutputFunc
	outdirs map[int]string
}

func (sp *fsSyncAttachable) Register(server *grpc.Server) {
	RegisterFileSendServer(server, sp)
}

func (sp *fsSyncAttachable) chooser(ctx context.Context) int {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0
	}
	values := md[keyExporterID]
	if len(values) == 0 {
		return 0
	}
	id, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return 0
	}
	return int(id)
}

func (sp *fsSyncAttachable) DiffCopy(stream FileSend_DiffCopyServer) (err error) {
	id := sp.chooser(stream.Context())
	if outdir, ok := sp.outdirs[id]; ok {
		return syncTargetDiffCopy(stream, outdir)
	}
	f, ok := sp.fs[id]
	if !ok {
		return errors.Errorf("exporter %d not found", id)
	}

	opts, _ := metadata.FromIncomingContext(stream.Context()) // if no metadata continue with empty object
	md := map[string]string{}
	for k, v := range opts {
		if strings.HasPrefix(k, keyExporterMetaPrefix) {
			md[strings.TrimPrefix(k, keyExporterMetaPrefix)] = strings.Join(v, ",")
		}
	}
	wc, err := f(md)
	if err != nil {
		return err
	}
	if wc == nil {
		return status.Errorf(codes.AlreadyExists, "target already exists")
	}
	defer func() {
		err1 := wc.Close()
		if err == nil {
			err = err1
		}
	}()
	return writeTargetFile(stream, wc)
}

func CopyToCaller(ctx context.Context, fs fsutil.FS, id int, c session.Caller, progress func(int, bool)) error {
	method := session.MethodURL(FileSend_ServiceDesc.ServiceName, "diffcopy")
	if !c.Supports(method) {
		return errors.Errorf("method %s not supported by the client", method)
	}

	client := NewFileSendClient(c.Conn())

	opts, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		opts = make(map[string][]string)
	}
	if existingVal, ok := opts[keyExporterID]; ok {
		bklog.G(ctx).Warnf("overwriting grpc metadata key %q from value %+v to %+v", keyExporterID, existingVal, id)
	}
	opts[keyExporterID] = []string{fmt.Sprint(id)}
	ctx = metadata.NewOutgoingContext(ctx, opts)

	cc, err := client.DiffCopy(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	return sendDiffCopy(cc, fs, progress)
}

func CopyFileWriter(ctx context.Context, md map[string]string, id int, c session.Caller) (io.WriteCloser, error) {
	method := session.MethodURL(FileSend_ServiceDesc.ServiceName, "diffcopy")
	if !c.Supports(method) {
		return nil, errors.Errorf("method %s not supported by the client", method)
	}

	client := NewFileSendClient(c.Conn())

	opts, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		opts = make(map[string][]string, len(md))
	}
	for k, v := range md {
		k := keyExporterMetaPrefix + k
		if existingVal, ok := opts[k]; ok {
			bklog.G(ctx).Warnf("overwriting grpc metadata key %q from value %+v to %+v", k, existingVal, v)
		}
		opts[k] = []string{v}
	}
	if existingVal, ok := opts[keyExporterID]; ok {
		bklog.G(ctx).Warnf("overwriting grpc metadata key %q from value %+v to %+v", keyExporterID, existingVal, id)
	}
	opts[keyExporterID] = []string{fmt.Sprint(id)}
	ctx = metadata.NewOutgoingContext(ctx, opts)

	cc, err := client.DiffCopy(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return newStreamWriter(cc), nil
}

type InvalidSessionError struct {
	err error
}

func (e InvalidSessionError) Error() string {
	return e.err.Error()
}

func (e InvalidSessionError) Unwrap() error {
	return e.err
}

func encodeOpts(opts map[string][]string) map[string][]string {
	md := make(map[string][]string, len(opts))
	for k, v := range opts {
		out, encoded := encodeStringForHeader(v)
		md[k] = out
		if encoded {
			md[k+"-encoded"] = []string{"1"}
		}
	}
	return md
}

func decodeOpts(opts map[string][]string) map[string][]string {
	md := make(map[string][]string, len(opts))
	for k, v := range opts {
		out := make([]string, len(v))
		var isEncoded bool
		if v, ok := opts[k+"-encoded"]; ok && len(v) > 0 {
			if b, _ := strconv.ParseBool(v[0]); b {
				isEncoded = true
			}
		}
		if isEncoded {
			for i, s := range v {
				out[i], _ = url.QueryUnescape(s)
			}
		} else {
			copy(out, v)
		}
		md[k] = out
	}
	return md
}

// encodeStringForHeader encodes a string value so it can be used in grpc header. This encoding
// is backwards compatible and avoids encoding ASCII characters.
func encodeStringForHeader(inputs []string) ([]string, bool) {
	var encode bool
loop:
	for _, input := range inputs {
		for _, runeVal := range input {
			// Only encode non-ASCII characters, and characters that have special
			// meaning during decoding.
			if runeVal > unicode.MaxASCII {
				encode = true
				break loop
			}
		}
	}
	if !encode {
		return inputs, false
	}
	for i, input := range inputs {
		inputs[i] = url.QueryEscape(input)
	}
	return inputs, true
}
