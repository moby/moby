package store

import (
	"io"

	"github.com/endophage/gotuf/data"
)

type targetsWalkFunc func(path string, meta data.FileMeta) error

type MetadataStore interface {
	GetMeta(name string, size int64) ([]byte, error)
	SetMeta(name string, blob []byte) error
	SetMultiMeta(map[string][]byte) error
}

type PublicKeyStore interface {
	GetKey(role string) ([]byte, error)
}

// [endophage] I'm of the opinion this should go away.
type TargetStore interface {
	WalkStagedTargets(paths []string, targetsFn targetsWalkFunc) error
}

type LocalStore interface {
	MetadataStore
	TargetStore
}

type RemoteStore interface {
	MetadataStore
	PublicKeyStore
	GetTarget(path string) (io.ReadCloser, error)
}
