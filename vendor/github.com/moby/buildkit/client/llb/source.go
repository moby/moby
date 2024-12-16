package llb

import (
	"context"
	_ "crypto/sha256" // for opencontainers/go-digest
	"encoding/json"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/distribution/reference"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/apicaps"
	"github.com/moby/buildkit/util/gitutil"
	"github.com/moby/buildkit/util/sshutil"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type SourceOp struct {
	cache       MarshalCache
	id          string
	attrs       map[string]string
	output      Output
	constraints Constraints
	err         error
}

func NewSource(id string, attrs map[string]string, c Constraints) *SourceOp {
	s := &SourceOp{
		id:          id,
		attrs:       attrs,
		constraints: c,
	}
	s.output = &output{vertex: s, platform: c.Platform}
	return s
}

func (s *SourceOp) Validate(ctx context.Context, c *Constraints) error {
	if s.err != nil {
		return s.err
	}
	if s.id == "" {
		return errors.Errorf("source identifier can't be empty")
	}
	return nil
}

func (s *SourceOp) Marshal(ctx context.Context, constraints *Constraints) (digest.Digest, []byte, *pb.OpMetadata, []*SourceLocation, error) {
	cache := s.cache.Acquire()
	defer cache.Release()

	if dgst, dt, md, srcs, err := cache.Load(constraints); err == nil {
		return dgst, dt, md, srcs, nil
	}

	if err := s.Validate(ctx, constraints); err != nil {
		return "", nil, nil, nil, err
	}

	if strings.HasPrefix(s.id, "local://") {
		if _, hasSession := s.attrs[pb.AttrLocalSessionID]; !hasSession {
			uid := s.constraints.LocalUniqueID
			if uid == "" {
				uid = constraints.LocalUniqueID
			}
			s.attrs[pb.AttrLocalUniqueID] = uid
			addCap(&s.constraints, pb.CapSourceLocalUnique)
		}
	}
	proto, md := MarshalConstraints(constraints, &s.constraints)

	proto.Op = &pb.Op_Source{
		Source: &pb.SourceOp{Identifier: s.id, Attrs: s.attrs},
	}

	if !platformSpecificSource(s.id) {
		proto.Platform = nil
	}

	dt, err := deterministicMarshal(proto)
	if err != nil {
		return "", nil, nil, nil, err
	}

	return cache.Store(dt, md, s.constraints.SourceLocations, constraints)
}

func (s *SourceOp) Output() Output {
	return s.output
}

func (s *SourceOp) Inputs() []Output {
	return nil
}

// Image returns a state that represents a docker image in a registry.
// Example:
//
//	st := llb.Image("busybox:latest")
func Image(ref string, opts ...ImageOption) State {
	r, err := reference.ParseNormalizedNamed(ref)
	if err == nil {
		r = reference.TagNameOnly(r)
		ref = r.String()
	}
	var info ImageInfo
	for _, opt := range opts {
		opt.SetImageOption(&info)
	}

	addCap(&info.Constraints, pb.CapSourceImage)

	attrs := map[string]string{}
	if info.resolveMode != 0 {
		attrs[pb.AttrImageResolveMode] = info.resolveMode.String()
		if info.resolveMode == ResolveModeForcePull {
			addCap(&info.Constraints, pb.CapSourceImageResolveMode) // only require cap for security enforced mode
		}
	}

	if info.RecordType != "" {
		attrs[pb.AttrImageRecordType] = info.RecordType
	}

	if ll := info.layerLimit; ll != nil {
		attrs[pb.AttrImageLayerLimit] = strconv.FormatInt(int64(*ll), 10)
		addCap(&info.Constraints, pb.CapSourceImageLayerLimit)
	}

	src := NewSource("docker-image://"+ref, attrs, info.Constraints) // controversial
	if err != nil {
		src.err = err
	} else if info.metaResolver != nil {
		if _, ok := r.(reference.Digested); ok || !info.resolveDigest {
			return NewState(src.Output()).Async(func(ctx context.Context, st State, c *Constraints) (State, error) {
				p := info.Constraints.Platform
				if p == nil {
					p = c.Platform
				}
				_, _, dt, err := info.metaResolver.ResolveImageConfig(ctx, ref, sourceresolver.Opt{
					Platform: p,
					ImageOpt: &sourceresolver.ResolveImageOpt{
						ResolveMode: info.resolveMode.String(),
					},
				})
				if err != nil {
					return State{}, err
				}
				return st.WithImageConfig(dt)
			})
		}
		return Scratch().Async(func(ctx context.Context, _ State, c *Constraints) (State, error) {
			p := info.Constraints.Platform
			if p == nil {
				p = c.Platform
			}
			ref, dgst, dt, err := info.metaResolver.ResolveImageConfig(context.TODO(), ref, sourceresolver.Opt{
				Platform: p,
				ImageOpt: &sourceresolver.ResolveImageOpt{
					ResolveMode: info.resolveMode.String(),
				},
			})
			if err != nil {
				return State{}, err
			}
			r, err := reference.ParseNormalizedNamed(ref)
			if err != nil {
				return State{}, err
			}
			if dgst != "" {
				r, err = reference.WithDigest(r, dgst)
				if err != nil {
					return State{}, err
				}
			}
			return NewState(NewSource("docker-image://"+r.String(), attrs, info.Constraints).Output()).WithImageConfig(dt)
		})
	}
	return NewState(src.Output())
}

type ImageOption interface {
	SetImageOption(*ImageInfo)
}

type imageOptionFunc func(*ImageInfo)

func (fn imageOptionFunc) SetImageOption(ii *ImageInfo) {
	fn(ii)
}

var MarkImageInternal = imageOptionFunc(func(ii *ImageInfo) {
	ii.RecordType = "internal"
})

type ResolveMode int

const (
	ResolveModeDefault ResolveMode = iota
	ResolveModeForcePull
	ResolveModePreferLocal
)

func (r ResolveMode) SetImageOption(ii *ImageInfo) {
	ii.resolveMode = r
}

func (r ResolveMode) String() string {
	switch r {
	case ResolveModeDefault:
		return pb.AttrImageResolveModeDefault
	case ResolveModeForcePull:
		return pb.AttrImageResolveModeForcePull
	case ResolveModePreferLocal:
		return pb.AttrImageResolveModePreferLocal
	default:
		return ""
	}
}

type ImageInfo struct {
	constraintsWrapper
	metaResolver  ImageMetaResolver
	resolveDigest bool
	resolveMode   ResolveMode
	layerLimit    *int
	RecordType    string
}

const (
	GitAuthHeaderKey = "GIT_AUTH_HEADER"
	GitAuthTokenKey  = "GIT_AUTH_TOKEN"
)

// Git returns a state that represents a git repository.
// Example:
//
//	st := llb.Git("https://github.com/moby/buildkit.git", "v0.11.6")
//
// The example fetches the v0.11.6 tag of the buildkit repository.
// You can also use a commit hash or a branch name.
//
// Other URL formats are supported such as "git@github.com:moby/buildkit.git", "git://...", "ssh://..."
// Formats that utilize SSH may need to supply credentials as a [GitOption].
// You may need to check the source code for a full list of supported formats.
//
// By default the git repository is cloned with `--depth=1` to reduce the amount of data downloaded.
// Additionally the ".git" directory is removed after the clone, you can keep ith with the [KeepGitDir] [GitOption].
func Git(url, ref string, opts ...GitOption) State {
	remote, err := gitutil.ParseURL(url)
	if errors.Is(err, gitutil.ErrUnknownProtocol) {
		url = "https://" + url
		remote, err = gitutil.ParseURL(url)
	}
	if remote != nil {
		url = remote.Remote
	}

	var id string
	if err != nil {
		// If we can't parse the URL, just use the full URL as the ID. The git
		// operation will fail later on.
		id = url
	} else {
		// We construct the ID manually here, so that we can create the same ID
		// for different protocols (e.g. https and ssh) that have the same
		// host/path/fragment combination.
		id = remote.Host + path.Join("/", remote.Path)
		if ref != "" {
			id += "#" + ref
		}
	}

	gi := &GitInfo{
		AuthHeaderSecret: GitAuthHeaderKey,
		AuthTokenSecret:  GitAuthTokenKey,
	}
	for _, o := range opts {
		o.SetGitOption(gi)
	}
	attrs := map[string]string{}
	if gi.KeepGitDir {
		attrs[pb.AttrKeepGitDir] = "true"
		addCap(&gi.Constraints, pb.CapSourceGitKeepDir)
	}
	if url != "" {
		attrs[pb.AttrFullRemoteURL] = url
		addCap(&gi.Constraints, pb.CapSourceGitFullURL)
	}
	if gi.AuthTokenSecret != "" {
		attrs[pb.AttrAuthTokenSecret] = gi.AuthTokenSecret
		if gi.addAuthCap {
			addCap(&gi.Constraints, pb.CapSourceGitHTTPAuth)
		}
	}
	if gi.AuthHeaderSecret != "" {
		attrs[pb.AttrAuthHeaderSecret] = gi.AuthHeaderSecret
		if gi.addAuthCap {
			addCap(&gi.Constraints, pb.CapSourceGitHTTPAuth)
		}
	}
	if remote != nil && remote.Scheme == gitutil.SSHProtocol {
		if gi.KnownSSHHosts != "" {
			attrs[pb.AttrKnownSSHHosts] = gi.KnownSSHHosts
		} else {
			keyscan, err := sshutil.SSHKeyScan(remote.Host)
			if err == nil {
				// best effort
				attrs[pb.AttrKnownSSHHosts] = keyscan
			}
		}
		addCap(&gi.Constraints, pb.CapSourceGitKnownSSHHosts)

		if gi.MountSSHSock == "" {
			attrs[pb.AttrMountSSHSock] = "default"
		} else {
			attrs[pb.AttrMountSSHSock] = gi.MountSSHSock
		}
		addCap(&gi.Constraints, pb.CapSourceGitMountSSHSock)
	}

	addCap(&gi.Constraints, pb.CapSourceGit)

	source := NewSource("git://"+id, attrs, gi.Constraints)
	return NewState(source.Output())
}

type GitOption interface {
	SetGitOption(*GitInfo)
}
type gitOptionFunc func(*GitInfo)

func (fn gitOptionFunc) SetGitOption(gi *GitInfo) {
	fn(gi)
}

type GitInfo struct {
	constraintsWrapper
	KeepGitDir       bool
	AuthTokenSecret  string
	AuthHeaderSecret string
	addAuthCap       bool
	KnownSSHHosts    string
	MountSSHSock     string
}

func KeepGitDir() GitOption {
	return gitOptionFunc(func(gi *GitInfo) {
		gi.KeepGitDir = true
	})
}

func AuthTokenSecret(v string) GitOption {
	return gitOptionFunc(func(gi *GitInfo) {
		gi.AuthTokenSecret = v
		gi.addAuthCap = true
	})
}

func AuthHeaderSecret(v string) GitOption {
	return gitOptionFunc(func(gi *GitInfo) {
		gi.AuthHeaderSecret = v
		gi.addAuthCap = true
	})
}

func KnownSSHHosts(key string) GitOption {
	key = strings.TrimSuffix(key, "\n")
	return gitOptionFunc(func(gi *GitInfo) {
		gi.KnownSSHHosts = gi.KnownSSHHosts + key + "\n"
	})
}

func MountSSHSock(sshID string) GitOption {
	return gitOptionFunc(func(gi *GitInfo) {
		gi.MountSSHSock = sshID
	})
}

// Scratch returns a state that represents an empty filesystem.
func Scratch() State {
	return NewState(nil)
}

// Local returns a state that represents a directory local to the client.
func Local(name string, opts ...LocalOption) State {
	gi := &LocalInfo{}

	for _, o := range opts {
		o.SetLocalOption(gi)
	}
	attrs := map[string]string{}
	if gi.SessionID != "" {
		attrs[pb.AttrLocalSessionID] = gi.SessionID
		addCap(&gi.Constraints, pb.CapSourceLocalSessionID)
	}
	if gi.IncludePatterns != "" {
		attrs[pb.AttrIncludePatterns] = gi.IncludePatterns
		addCap(&gi.Constraints, pb.CapSourceLocalIncludePatterns)
	}
	if gi.FollowPaths != "" {
		attrs[pb.AttrFollowPaths] = gi.FollowPaths
		addCap(&gi.Constraints, pb.CapSourceLocalFollowPaths)
	}
	if gi.ExcludePatterns != "" {
		attrs[pb.AttrExcludePatterns] = gi.ExcludePatterns
		addCap(&gi.Constraints, pb.CapSourceLocalExcludePatterns)
	}
	if gi.SharedKeyHint != "" {
		attrs[pb.AttrSharedKeyHint] = gi.SharedKeyHint
		addCap(&gi.Constraints, pb.CapSourceLocalSharedKeyHint)
	}
	if gi.Differ.Type != "" {
		attrs[pb.AttrLocalDiffer] = string(gi.Differ.Type)
		if gi.Differ.Required {
			addCap(&gi.Constraints, pb.CapSourceLocalDiffer)
		}
	}

	addCap(&gi.Constraints, pb.CapSourceLocal)

	source := NewSource("local://"+name, attrs, gi.Constraints)
	return NewState(source.Output())
}

type LocalOption interface {
	SetLocalOption(*LocalInfo)
}

type localOptionFunc func(*LocalInfo)

func (fn localOptionFunc) SetLocalOption(li *LocalInfo) {
	fn(li)
}

func SessionID(id string) LocalOption {
	return localOptionFunc(func(li *LocalInfo) {
		li.SessionID = id
	})
}

func IncludePatterns(p []string) LocalOption {
	return localOptionFunc(func(li *LocalInfo) {
		if len(p) == 0 {
			li.IncludePatterns = ""
			return
		}
		dt, _ := json.Marshal(p) // empty on error
		li.IncludePatterns = string(dt)
	})
}

func FollowPaths(p []string) LocalOption {
	return localOptionFunc(func(li *LocalInfo) {
		if len(p) == 0 {
			li.FollowPaths = ""
			return
		}
		dt, _ := json.Marshal(p) // empty on error
		li.FollowPaths = string(dt)
	})
}

func ExcludePatterns(p []string) LocalOption {
	return localOptionFunc(func(li *LocalInfo) {
		if len(p) == 0 {
			li.ExcludePatterns = ""
			return
		}
		dt, _ := json.Marshal(p) // empty on error
		li.ExcludePatterns = string(dt)
	})
}

func SharedKeyHint(h string) LocalOption {
	return localOptionFunc(func(li *LocalInfo) {
		li.SharedKeyHint = h
	})
}

func Differ(t DiffType, required bool) LocalOption {
	return localOptionFunc(func(li *LocalInfo) {
		li.Differ = DifferInfo{
			Type:     t,
			Required: required,
		}
	})
}

func OCILayout(ref string, opts ...OCILayoutOption) State {
	gi := &OCILayoutInfo{}

	for _, o := range opts {
		o.SetOCILayoutOption(gi)
	}
	attrs := map[string]string{}
	if gi.sessionID != "" {
		attrs[pb.AttrOCILayoutSessionID] = gi.sessionID
	}
	if gi.storeID != "" {
		attrs[pb.AttrOCILayoutStoreID] = gi.storeID
	}
	if gi.layerLimit != nil {
		attrs[pb.AttrOCILayoutLayerLimit] = strconv.FormatInt(int64(*gi.layerLimit), 10)
	}

	addCap(&gi.Constraints, pb.CapSourceOCILayout)

	source := NewSource("oci-layout://"+ref, attrs, gi.Constraints)
	return NewState(source.Output())
}

type OCILayoutOption interface {
	SetOCILayoutOption(*OCILayoutInfo)
}

type ociLayoutOptionFunc func(*OCILayoutInfo)

func (fn ociLayoutOptionFunc) SetOCILayoutOption(li *OCILayoutInfo) {
	fn(li)
}

func OCIStore(sessionID string, storeID string) OCILayoutOption {
	return ociLayoutOptionFunc(func(oi *OCILayoutInfo) {
		oi.sessionID = sessionID
		oi.storeID = storeID
	})
}

func OCILayerLimit(limit int) OCILayoutOption {
	return ociLayoutOptionFunc(func(oi *OCILayoutInfo) {
		oi.layerLimit = &limit
	})
}

type OCILayoutInfo struct {
	constraintsWrapper
	sessionID  string
	storeID    string
	layerLimit *int
}

type DiffType string

const (
	// DiffNone will do no file comparisons, all files in the Local source will
	// be retransmitted.
	DiffNone DiffType = pb.AttrLocalDifferNone
	// DiffMetadata will compare file metadata (size, modified time, mode, owner,
	// group, device and link name) to determine if the files in the Local source need
	// to be retransmitted.  This is the default behavior.
	DiffMetadata DiffType = pb.AttrLocalDifferMetadata
)

type DifferInfo struct {
	Type     DiffType
	Required bool
}

type LocalInfo struct {
	constraintsWrapper
	SessionID       string
	IncludePatterns string
	ExcludePatterns string
	FollowPaths     string
	SharedKeyHint   string
	Differ          DifferInfo
}

func HTTP(url string, opts ...HTTPOption) State {
	hi := &HTTPInfo{}
	for _, o := range opts {
		o.SetHTTPOption(hi)
	}
	attrs := map[string]string{}
	if hi.Checksum != "" {
		attrs[pb.AttrHTTPChecksum] = hi.Checksum.String()
		addCap(&hi.Constraints, pb.CapSourceHTTPChecksum)
	}
	if hi.Filename != "" {
		attrs[pb.AttrHTTPFilename] = hi.Filename
	}
	if hi.Perm != 0 {
		attrs[pb.AttrHTTPPerm] = "0" + strconv.FormatInt(int64(hi.Perm), 8)
		addCap(&hi.Constraints, pb.CapSourceHTTPPerm)
	}
	if hi.UID != 0 {
		attrs[pb.AttrHTTPUID] = strconv.Itoa(hi.UID)
		addCap(&hi.Constraints, pb.CapSourceHTTPUIDGID)
	}
	if hi.GID != 0 {
		attrs[pb.AttrHTTPGID] = strconv.Itoa(hi.GID)
		addCap(&hi.Constraints, pb.CapSourceHTTPUIDGID)
	}

	addCap(&hi.Constraints, pb.CapSourceHTTP)
	source := NewSource(url, attrs, hi.Constraints)
	return NewState(source.Output())
}

type HTTPInfo struct {
	constraintsWrapper
	Checksum digest.Digest
	Filename string
	Perm     int
	UID      int
	GID      int
}

type HTTPOption interface {
	SetHTTPOption(*HTTPInfo)
}

type httpOptionFunc func(*HTTPInfo)

func (fn httpOptionFunc) SetHTTPOption(hi *HTTPInfo) {
	fn(hi)
}

func Checksum(dgst digest.Digest) HTTPOption {
	return httpOptionFunc(func(hi *HTTPInfo) {
		hi.Checksum = dgst
	})
}

func Chmod(perm os.FileMode) HTTPOption {
	return httpOptionFunc(func(hi *HTTPInfo) {
		hi.Perm = int(perm) & 0777
	})
}

func Filename(name string) HTTPOption {
	return httpOptionFunc(func(hi *HTTPInfo) {
		hi.Filename = name
	})
}

func Chown(uid, gid int) HTTPOption {
	return httpOptionFunc(func(hi *HTTPInfo) {
		hi.UID = uid
		hi.GID = gid
	})
}

func platformSpecificSource(id string) bool {
	return strings.HasPrefix(id, "docker-image://") || strings.HasPrefix(id, "oci-layout://")
}

func addCap(c *Constraints, id apicaps.CapID) {
	if c.Metadata.Caps == nil {
		c.Metadata.Caps = make(map[apicaps.CapID]bool)
	}
	c.Metadata.Caps[id] = true
}
