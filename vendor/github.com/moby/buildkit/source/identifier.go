package source

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/containerd/containerd/reference"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var (
	errInvalid  = errors.New("invalid")
	errNotFound = errors.New("not found")
)

type ResolveMode int

const (
	ResolveModeDefault ResolveMode = iota
	ResolveModeForcePull
	ResolveModePreferLocal
)

const (
	DockerImageScheme = "docker-image"
	GitScheme         = "git"
	LocalScheme       = "local"
	HttpScheme        = "http"
	HttpsScheme       = "https"
)

type Identifier interface {
	ID() string // until sources are in process this string comparison could be avoided
}

func FromString(s string) (Identifier, error) {
	// TODO: improve this
	parts := strings.SplitN(s, "://", 2)
	if len(parts) != 2 {
		return nil, errors.Wrapf(errInvalid, "failed to parse %s", s)
	}

	switch parts[0] {
	case DockerImageScheme:
		return NewImageIdentifier(parts[1])
	case GitScheme:
		return NewGitIdentifier(parts[1])
	case LocalScheme:
		return NewLocalIdentifier(parts[1])
	case HttpsScheme:
		return NewHttpIdentifier(parts[1], true)
	case HttpScheme:
		return NewHttpIdentifier(parts[1], false)
	default:
		return nil, errors.Wrapf(errNotFound, "unknown schema %s", parts[0])
	}
}

func FromLLB(op *pb.Op_Source, platform *pb.Platform) (Identifier, error) {
	id, err := FromString(op.Source.Identifier)
	if err != nil {
		return nil, err
	}

	if id, ok := id.(*ImageIdentifier); ok {
		if platform != nil {
			id.Platform = &specs.Platform{
				OS:           platform.OS,
				Architecture: platform.Architecture,
				Variant:      platform.Variant,
				OSVersion:    platform.OSVersion,
				OSFeatures:   platform.OSFeatures,
			}
		}
		for k, v := range op.Source.Attrs {
			switch k {
			case pb.AttrImageResolveMode:
				rm, err := ParseImageResolveMode(v)
				if err != nil {
					return nil, err
				}
				id.ResolveMode = rm
			case pb.AttrImageRecordType:
				rt, err := parseImageRecordType(v)
				if err != nil {
					return nil, err
				}
				id.RecordType = rt
			}
		}
	}
	if id, ok := id.(*GitIdentifier); ok {
		for k, v := range op.Source.Attrs {
			switch k {
			case pb.AttrKeepGitDir:
				if v == "true" {
					id.KeepGitDir = true
				}
			case pb.AttrFullRemoteURL:
				id.Remote = v
			}
		}
	}
	if id, ok := id.(*LocalIdentifier); ok {
		for k, v := range op.Source.Attrs {
			switch k {
			case pb.AttrLocalSessionID:
				id.SessionID = v
				if p := strings.SplitN(v, ":", 2); len(p) == 2 {
					id.Name = p[0] + "-" + id.Name
					id.SessionID = p[1]
				}
			case pb.AttrIncludePatterns:
				var patterns []string
				if err := json.Unmarshal([]byte(v), &patterns); err != nil {
					return nil, err
				}
				id.IncludePatterns = patterns
			case pb.AttrExcludePatterns:
				var patterns []string
				if err := json.Unmarshal([]byte(v), &patterns); err != nil {
					return nil, err
				}
				id.ExcludePatterns = patterns
			case pb.AttrFollowPaths:
				var paths []string
				if err := json.Unmarshal([]byte(v), &paths); err != nil {
					return nil, err
				}
				id.FollowPaths = paths
			case pb.AttrSharedKeyHint:
				id.SharedKeyHint = v
			}
		}
	}
	if id, ok := id.(*HttpIdentifier); ok {
		for k, v := range op.Source.Attrs {
			switch k {
			case pb.AttrHTTPChecksum:
				dgst, err := digest.Parse(v)
				if err != nil {
					return nil, err
				}
				id.Checksum = dgst
			case pb.AttrHTTPFilename:
				id.Filename = v
			case pb.AttrHTTPPerm:
				i, err := strconv.ParseInt(v, 0, 64)
				if err != nil {
					return nil, err
				}
				id.Perm = int(i)
			case pb.AttrHTTPUID:
				i, err := strconv.ParseInt(v, 0, 64)
				if err != nil {
					return nil, err
				}
				id.UID = int(i)
			case pb.AttrHTTPGID:
				i, err := strconv.ParseInt(v, 0, 64)
				if err != nil {
					return nil, err
				}
				id.GID = int(i)
			}
		}
	}
	return id, nil
}

type ImageIdentifier struct {
	Reference   reference.Spec
	Platform    *specs.Platform
	ResolveMode ResolveMode
	RecordType  client.UsageRecordType
}

func NewImageIdentifier(str string) (*ImageIdentifier, error) {
	ref, err := reference.Parse(str)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if ref.Object == "" {
		return nil, errors.WithStack(reference.ErrObjectRequired)
	}
	return &ImageIdentifier{Reference: ref}, nil
}

func (_ *ImageIdentifier) ID() string {
	return DockerImageScheme
}

type LocalIdentifier struct {
	Name            string
	SessionID       string
	IncludePatterns []string
	ExcludePatterns []string
	FollowPaths     []string
	SharedKeyHint   string
}

func NewLocalIdentifier(str string) (*LocalIdentifier, error) {
	return &LocalIdentifier{Name: str}, nil
}

func (*LocalIdentifier) ID() string {
	return LocalScheme
}

func NewHttpIdentifier(str string, tls bool) (*HttpIdentifier, error) {
	proto := "https://"
	if !tls {
		proto = "http://"
	}
	return &HttpIdentifier{TLS: tls, URL: proto + str}, nil
}

type HttpIdentifier struct {
	TLS      bool
	URL      string
	Checksum digest.Digest
	Filename string
	Perm     int
	UID      int
	GID      int
}

func (_ *HttpIdentifier) ID() string {
	return HttpsScheme
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

func ParseImageResolveMode(v string) (ResolveMode, error) {
	switch v {
	case pb.AttrImageResolveModeDefault, "":
		return ResolveModeDefault, nil
	case pb.AttrImageResolveModeForcePull:
		return ResolveModeForcePull, nil
	case pb.AttrImageResolveModePreferLocal:
		return ResolveModePreferLocal, nil
	default:
		return 0, errors.Errorf("invalid resolvemode: %s", v)
	}
}

func parseImageRecordType(v string) (client.UsageRecordType, error) {
	switch client.UsageRecordType(v) {
	case "", client.UsageRecordTypeRegular:
		return client.UsageRecordTypeRegular, nil
	case client.UsageRecordTypeInternal:
		return client.UsageRecordTypeInternal, nil
	case client.UsageRecordTypeFrontend:
		return client.UsageRecordTypeFrontend, nil
	default:
		return "", errors.Errorf("invalid record type %s", v)
	}
}
