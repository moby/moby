package store

import (
	"io"

	"github.com/docker/notary/tuf/data"
)

type targetsWalkFunc func(path string, meta data.FileMeta) error

// MetadataStore must be implemented by anything that intends to interact
// with a store of TUF files
type MetadataStore interface {
	GetMeta(name string, size int64) ([]byte, error)
	SetMeta(name string, blob []byte) error
	SetMultiMeta(map[string][]byte) error
}

// PublicKeyStore must be implemented by a key service
type PublicKeyStore interface {
	GetKey(role string) ([]byte, error)
}

// TargetStore represents a collection of targets that can be walked similarly
// to walking a directory, passing a callback that receives the path and meta
// for each target
type TargetStore interface {
	WalkStagedTargets(paths []string, targetsFn targetsWalkFunc) error
}

// LocalStore represents a local TUF sture
type LocalStore interface {
	MetadataStore
	TargetStore
}

// RemoteStore is similar to LocalStore with the added expectation that it should
// provide a way to download targets once located
type RemoteStore interface {
	MetadataStore
	PublicKeyStore
	GetTarget(path string) (io.ReadCloser, error)
}
