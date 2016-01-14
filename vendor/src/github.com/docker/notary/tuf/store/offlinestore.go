package store

import (
	"io"
)

// ErrOffline is used to indicate we are operating offline
type ErrOffline struct{}

func (e ErrOffline) Error() string {
	return "client is offline"
}

var err = ErrOffline{}

// OfflineStore is to be used as a placeholder for a nil store. It simply
// return ErrOffline for every operation
type OfflineStore struct{}

// GetMeta return ErrOffline
func (es OfflineStore) GetMeta(name string, size int64) ([]byte, error) {
	return nil, err
}

// SetMeta return ErrOffline
func (es OfflineStore) SetMeta(name string, blob []byte) error {
	return err
}

// SetMultiMeta return ErrOffline
func (es OfflineStore) SetMultiMeta(map[string][]byte) error {
	return err
}

// GetKey return ErrOffline
func (es OfflineStore) GetKey(role string) ([]byte, error) {
	return nil, err
}

// GetTarget return ErrOffline
func (es OfflineStore) GetTarget(path string) (io.ReadCloser, error) {
	return nil, err
}
