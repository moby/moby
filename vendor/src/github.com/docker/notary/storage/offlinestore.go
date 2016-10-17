package storage

// ErrOffline is used to indicate we are operating offline
type ErrOffline struct{}

func (e ErrOffline) Error() string {
	return "client is offline"
}

var err = ErrOffline{}

// OfflineStore is to be used as a placeholder for a nil store. It simply
// returns ErrOffline for every operation
type OfflineStore struct{}

// GetSized returns ErrOffline
func (es OfflineStore) GetSized(name string, size int64) ([]byte, error) {
	return nil, err
}

// Set returns ErrOffline
func (es OfflineStore) Set(name string, blob []byte) error {
	return err
}

// SetMulti returns ErrOffline
func (es OfflineStore) SetMulti(map[string][]byte) error {
	return err
}

// Remove returns ErrOffline
func (es OfflineStore) Remove(name string) error {
	return err
}

// GetKey returns ErrOffline
func (es OfflineStore) GetKey(role string) ([]byte, error) {
	return nil, err
}

// RotateKey returns ErrOffline
func (es OfflineStore) RotateKey(role string) ([]byte, error) {
	return nil, err
}

// RemoveAll return ErrOffline
func (es OfflineStore) RemoveAll() error {
	return err
}

// Location returns a human readable name for the storage location
func (es OfflineStore) Location() string {
	return "offline"
}
