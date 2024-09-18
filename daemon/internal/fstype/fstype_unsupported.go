//go:build !linux

package fstype

// getFSMagic returns the filesystem id given the path.
func getFSMagic(rootpath string) (FsMagic, error) {
	return FsMagicUnsupported, nil
}
