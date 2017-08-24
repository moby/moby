// +build !windows

package layer

import "runtime"

// SetOS writes the "os" file to the layer filestore
func (fm *fileMetadataTransaction) SetOS(os string) error {
	return nil
}

// GetOS reads the "os" file from the layer filestore
func (fms *fileMetadataStore) GetOS(layer ChainID) (string, error) {
	return runtime.GOOS, nil
}
