package storage

// NoSizeLimit is represented as -1 for arguments to GetMeta
const NoSizeLimit int64 = -1

// MetadataStore must be implemented by anything that intends to interact
// with a store of TUF files
type MetadataStore interface {
	GetSized(name string, size int64) ([]byte, error)
	Set(name string, blob []byte) error
	SetMulti(map[string][]byte) error
	RemoveAll() error
	Remove(name string) error
}

// PublicKeyStore must be implemented by a key service
type PublicKeyStore interface {
	GetKey(role string) ([]byte, error)
	RotateKey(role string) ([]byte, error)
}

// RemoteStore is similar to LocalStore with the added expectation that it should
// provide a way to download targets once located
type RemoteStore interface {
	MetadataStore
	PublicKeyStore
}

// Bootstrapper is a thing that can set itself up
type Bootstrapper interface {
	// Bootstrap instructs a configured Bootstrapper to perform
	// its setup operations.
	Bootstrap() error
}
