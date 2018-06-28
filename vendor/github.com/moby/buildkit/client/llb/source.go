package llb

import (
	"context"
	_ "crypto/sha256"
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type SourceOp struct {
	MarshalCache
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

func (s *SourceOp) Validate() error {
	if s.err != nil {
		return s.err
	}
	if s.id == "" {
		return errors.Errorf("source identifier can't be empty")
	}
	return nil
}

func (s *SourceOp) Marshal(constraints *Constraints) (digest.Digest, []byte, *pb.OpMetadata, error) {
	if s.Cached(constraints) {
		return s.Load()
	}
	if err := s.Validate(); err != nil {
		return "", nil, nil, err
	}

	proto, md := MarshalConstraints(constraints, &s.constraints)

	proto.Op = &pb.Op_Source{
		Source: &pb.SourceOp{Identifier: s.id, Attrs: s.attrs},
	}
	dt, err := proto.Marshal()
	if err != nil {
		return "", nil, nil, err
	}

	s.Store(dt, md, constraints)
	return s.Load()
}

func (s *SourceOp) Output() Output {
	return s.output
}

func (s *SourceOp) Inputs() []Output {
	return nil
}

func Image(ref string, opts ...ImageOption) State {
	r, err := reference.ParseNormalizedNamed(ref)
	if err == nil {
		ref = reference.TagNameOnly(r).String()
	}
	var info ImageInfo
	for _, opt := range opts {
		opt.SetImageOption(&info)
	}
	src := NewSource("docker-image://"+ref, nil, info.Constraints) // controversial
	if err != nil {
		src.err = err
	}
	if info.metaResolver != nil {
		_, dt, err := info.metaResolver.ResolveImageConfig(context.TODO(), ref, info.Constraints.Platform)
		if err != nil {
			src.err = err
		} else {
			var img struct {
				Config struct {
					Env        []string `json:"Env,omitempty"`
					WorkingDir string   `json:"WorkingDir,omitempty"`
					User       string   `json:"User,omitempty"`
				} `json:"config,omitempty"`
			}
			if err := json.Unmarshal(dt, &img); err != nil {
				src.err = err
			} else {
				st := NewState(src.Output())
				for _, env := range img.Config.Env {
					parts := strings.SplitN(env, "=", 2)
					if len(parts[0]) > 0 {
						var v string
						if len(parts) > 1 {
							v = parts[1]
						}
						st = st.AddEnv(parts[0], v)
					}
				}
				st = st.Dir(img.Config.WorkingDir)
				return st
			}
		}
	}
	return NewState(src.Output())
}

type ImageOption interface {
	SetImageOption(*ImageInfo)
}

type ImageOptionFunc func(*ImageInfo)

func (fn ImageOptionFunc) SetImageOption(ii *ImageInfo) {
	fn(ii)
}

type ImageInfo struct {
	constraintsWrapper
	metaResolver ImageMetaResolver
}

func Git(remote, ref string, opts ...GitOption) State {
	url := ""

	for _, prefix := range []string{
		"http://", "https://", "git://", "git@",
	} {
		if strings.HasPrefix(remote, prefix) {
			url = strings.Split(remote, "#")[0]
			remote = strings.TrimPrefix(remote, prefix)
		}
	}

	id := remote

	if ref != "" {
		id += "#" + ref
	}

	gi := &GitInfo{}
	for _, o := range opts {
		o.SetGitOption(gi)
	}
	attrs := map[string]string{}
	if gi.KeepGitDir {
		attrs[pb.AttrKeepGitDir] = "true"
	}
	if url != "" {
		attrs[pb.AttrFullRemoteURL] = url
	}
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
	KeepGitDir bool
}

func KeepGitDir() GitOption {
	return gitOptionFunc(func(gi *GitInfo) {
		gi.KeepGitDir = true
	})
}

func Scratch() State {
	return NewState(nil)
}

func Local(name string, opts ...LocalOption) State {
	gi := &LocalInfo{}

	for _, o := range opts {
		o.SetLocalOption(gi)
	}
	attrs := map[string]string{}
	if gi.SessionID != "" {
		attrs[pb.AttrLocalSessionID] = gi.SessionID
	}
	if gi.IncludePatterns != "" {
		attrs[pb.AttrIncludePatterns] = gi.IncludePatterns
	}
	if gi.FollowPaths != "" {
		attrs[pb.AttrFollowPaths] = gi.FollowPaths
	}
	if gi.ExcludePatterns != "" {
		attrs[pb.AttrExcludePatterns] = gi.ExcludePatterns
	}
	if gi.SharedKeyHint != "" {
		attrs[pb.AttrSharedKeyHint] = gi.SharedKeyHint
	}

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

type LocalInfo struct {
	constraintsWrapper
	SessionID       string
	IncludePatterns string
	ExcludePatterns string
	FollowPaths     string
	SharedKeyHint   string
}

func HTTP(url string, opts ...HTTPOption) State {
	hi := &HTTPInfo{}
	for _, o := range opts {
		o.SetHTTPOption(hi)
	}
	attrs := map[string]string{}
	if hi.Checksum != "" {
		attrs[pb.AttrHTTPChecksum] = hi.Checksum.String()
	}
	if hi.Filename != "" {
		attrs[pb.AttrHTTPFilename] = hi.Filename
	}
	if hi.Perm != 0 {
		attrs[pb.AttrHTTPPerm] = "0" + strconv.FormatInt(int64(hi.Perm), 8)
	}
	if hi.UID != 0 {
		attrs[pb.AttrHTTPUID] = strconv.Itoa(hi.UID)
	}
	if hi.UID != 0 {
		attrs[pb.AttrHTTPGID] = strconv.Itoa(hi.GID)
	}

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
