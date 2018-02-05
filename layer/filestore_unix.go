// +build !windows

package layer // import "github.com/docker/docker/layer"

import "runtime"

// setOS writes the "os" file to the layer filestore
func (fm *fileMetadataTransaction) setOS(os string) error {
	return nil
}

// getOS reads the "os" file from the layer filestore
func (fms *fileMetadataStore) getOS(layer ChainID) (string, error) {
	return runtime.GOOS, nil
}
