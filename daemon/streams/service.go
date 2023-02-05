package streams

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	router "github.com/docker/docker/api/server/router/streams"
	"github.com/docker/docker/api/types/streams"
	"github.com/docker/docker/daemon/names"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stringid"
	"github.com/opencontainers/go-digest"
	"go.etcd.io/bbolt"
)

var _ router.Backend = (*Service)(nil)

type Service struct {
	store    *Store
	execRoot string
}

func NewService(ctx context.Context, root, execRoot string) (*Service, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("could not create streams root dir: %w", err)
	}
	if err := os.MkdirAll(execRoot, 0o700); err != nil {
		return nil, fmt.Errorf("could not create streams exec root dir: %w", err)
	}
	dbpath := filepath.Join(root, "streams.db")

	timeout := 10 * time.Second

	if deadline, ok := ctx.Deadline(); ok {
		timeout = deadline.Sub(time.Now())
	}

	db, err := bbolt.Open(dbpath, 0600, &bbolt.Options{Timeout: timeout})
	if err != nil {
		return nil, fmt.Errorf("could not create streams k/v store: %w", err)
	}
	return &Service{store: NewStore(db), execRoot: execRoot}, nil
}

func (s *Service) Create(ctx context.Context, id string, spec streams.Spec) (*streams.Stream, error) {
	if id == "" {
		id = stringid.GenerateRandomID()
	}

	if !names.RestrictedNamePattern.MatchString(id) {
		return nil, errdefs.InvalidParameter(fmt.Errorf("invalid stream ID: id must match pattern %q", names.RestrictedNameChars))
	}

	if spec.Protocol == "" {
		spec.Protocol = streams.ProtocolPipe
	}

	if spec.Protocol == streams.ProtocolPipe && (spec.PipeConfig == nil || spec.PipeConfig.Path == "") {
		key := digest.FromBytes([]byte(id))
		spec.PipeConfig = &streams.PipeConfig{Path: filepath.Join(s.execRoot, key.Encoded())}
	}

	if err := Validate(spec); err != nil {
		return nil, err
	}

	stream := streams.Stream{ID: id, Spec: spec}
	if err := s.store.Create(stream); err != nil {
		return nil, err
	}
	return &stream, nil
}

func (s *Service) Get(ctx context.Context, id string) (*streams.Stream, error) {
	return s.store.Get(id)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.store.Delete(id)
}

func Validate(spec streams.Spec) error {
	switch spec.Protocol {
	case streams.ProtocolPipe:
		if spec.TCPConnectConfig != nil {
			return errdefs.InvalidParameter(errors.New("tcp connect config must not be set for pipe protocol"))
		}
		if spec.UnixConnectConfig != nil {
			return errdefs.InvalidParameter(errors.New("unix connect config must not be set for pipe protocol"))
		}
		if spec.PipeConfig == nil || spec.PipeConfig.Path == "" {
			return errdefs.InvalidParameter(errors.New("missing pipe config"))
		}
	case streams.ProtocolTCPConnect:
		if spec.PipeConfig != nil {
			return errdefs.InvalidParameter(errors.New("pipe config must not be set for tcp connect protocol"))
		}
		if spec.UnixConnectConfig != nil {
			return errdefs.InvalidParameter(errors.New("unix connect config must not be set for tcp connect protocol"))
		}
		if spec.TCPConnectConfig == nil {
			return errdefs.InvalidParameter(errors.New("missing tcp connect config"))
		}
	case streams.ProtocolUnixConnect:
		if spec.PipeConfig != nil {
			return errdefs.InvalidParameter(errors.New("pipe config must not be set for unix connect protocol"))
		}
		if spec.TCPConnectConfig != nil {
			return errdefs.InvalidParameter(errors.New("tcp connect config must not be set for unix connect protocol"))
		}
		if spec.UnixConnectConfig == nil {
			return errdefs.InvalidParameter(errors.New("missing unix connect config"))
		}
	default:
		return errdefs.InvalidParameter(errors.New("unknown protocol"))
	}
	return nil
}

func (s *Service) Close() error {
	return s.store.Close()
}
