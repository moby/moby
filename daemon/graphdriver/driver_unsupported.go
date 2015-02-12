// +build !linux

package graphdriver

func GetFSMagic(rootpath string) (FsMagic, error) {
	return FsMagicUnsupported, nil
}
