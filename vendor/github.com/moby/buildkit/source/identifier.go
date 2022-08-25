package source

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/containerd/containerd/reference"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/solver/pb"
	srctypes "github.com/moby/buildkit/source/types"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
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
	case srctypes.DockerImageScheme:
		return NewImageIdentifier(parts[1])
	case srctypes.GitScheme:
		return NewGitIdentifier(parts[1])
	case srctypes.LocalScheme:
		return NewLocalIdentifier(parts[1])
	case srctypes.HTTPSScheme:
		return NewHTTPIdentifier(parts[1], true)
	case srctypes.HTTPScheme:
		return NewHTTPIdentifier(parts[1], false)
	case srctypes.OCIScheme:
		return NewOCIIdentifier(parts[1])
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
			id.Platform = &ocispecs.Platform{
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
			case pb.AttrImageLayerLimit:
				l, err := strconv.Atoi(v)
				if err != nil {
					return nil, errors.Wrapf(err, "invalid layer limit %s", v)
				}
				if l <= 0 {
					return nil, errors.Errorf("invalid layer limit %s", v)
				}
				id.LayerLimit = &l
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
				if !isGitTransport(v) {
					v = "https://" + v
				}
				id.Remote = v
			case pb.AttrAuthHeaderSecret:
				id.AuthHeaderSecret = v
			case pb.AttrAuthTokenSecret:
				id.AuthTokenSecret = v
			case pb.AttrKnownSSHHosts:
				id.KnownSSHHosts = v
			case pb.AttrMountSSHSock:
				id.MountSSHSock = v
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
			case pb.AttrLocalDiffer:
				switch v {
				case pb.AttrLocalDifferMetadata, "":
					id.Differ = fsutil.DiffMetadata
				case pb.AttrLocalDifferNone:
					id.Differ = fsutil.DiffNone
				}
			}
		}
	}
	if id, ok := id.(*HTTPIdentifier); ok {
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
	if id, ok := id.(*OCIIdentifier); ok {
		if platform != nil {
			id.Platform = &ocispecs.Platform{
				OS:           platform.OS,
				Architecture: platform.Architecture,
				Variant:      platform.Variant,
				OSVersion:    platform.OSVersion,
				OSFeatures:   platform.OSFeatures,
			}
		}
		for k, v := range op.Source.Attrs {
			switch k {
			case pb.AttrOCILayoutSessionID:
				id.SessionID = v
				if p := strings.SplitN(v, ":", 2); len(p) == 2 {
					id.Name = p[0] + "-" + id.Name
					id.SessionID = p[1]
				}
			case pb.AttrOCILayoutLayerLimit:
				l, err := strconv.Atoi(v)
				if err != nil {
					return nil, errors.Wrapf(err, "invalid layer limit %s", v)
				}
				if l <= 0 {
					return nil, errors.Errorf("invalid layer limit %s", v)
				}
				id.LayerLimit = &l
			}
		}
	}
	return id, nil
}

type ImageIdentifier struct {
	Reference   reference.Spec
	Platform    *ocispecs.Platform
	ResolveMode ResolveMode
	RecordType  client.UsageRecordType
	LayerLimit  *int
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

func (*ImageIdentifier) ID() string {
	return srctypes.DockerImageScheme
}

type LocalIdentifier struct {
	Name            string
	SessionID       string
	IncludePatterns []string
	ExcludePatterns []string
	FollowPaths     []string
	SharedKeyHint   string
	Differ          fsutil.DiffType
}

func NewLocalIdentifier(str string) (*LocalIdentifier, error) {
	return &LocalIdentifier{Name: str}, nil
}

func (*LocalIdentifier) ID() string {
	return srctypes.LocalScheme
}

func NewHTTPIdentifier(str string, tls bool) (*HTTPIdentifier, error) {
	proto := "https://"
	if !tls {
		proto = "http://"
	}
	return &HTTPIdentifier{TLS: tls, URL: proto + str}, nil
}

type HTTPIdentifier struct {
	TLS      bool
	URL      string
	Checksum digest.Digest
	Filename string
	Perm     int
	UID      int
	GID      int
}

func (*HTTPIdentifier) ID() string {
	return srctypes.HTTPSScheme
}

type OCIIdentifier struct {
	Name       string
	Manifest   digest.Digest
	Platform   *ocispecs.Platform
	SessionID  string
	LayerLimit *int
}

func NewOCIIdentifier(str string) (*OCIIdentifier, error) {
	// OCI identifier arg is of the format: path@hash
	parts := strings.SplitN(str, "@", 2)
	if len(parts) != 2 {
		return nil, errors.New("OCI must be in format of storeID@manifest-hash")
	}
	dig, err := digest.Parse(parts[1])
	if err != nil {
		return nil, errors.Wrap(err, "OCI must be in format of storeID@manifest-hash, invalid digest")
	}
	return &OCIIdentifier{Name: parts[0], Manifest: dig}, nil
}

func (*OCIIdentifier) ID() string {
	return srctypes.OCIScheme
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
