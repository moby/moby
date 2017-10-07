// +build !windows

package layer

// SetOS writes the "os" file to the layer filestore
func (fm *fileMetadataTransaction) SetOS(os OS) error {
	return nil
}

// GetOS reads the "os" file from the layer filestore
func (fms *fileMetadataStore) GetOS(layer ChainID) (OS, error) {
	return "", nil
}
