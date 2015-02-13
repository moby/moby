// +build windows

package graphdriver

var (
	// Slice of drivers that should be used in an order
	priority = []string{
		"windows",
	}

	FsNames = map[FsMagic]string{
		FsMagicUnsupported: "unsupported",
	}
)

func GetFSMagic(rootpath string) (FsMagic, error) {
	return 0, nil
}
