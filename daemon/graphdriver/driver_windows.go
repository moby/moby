package graphdriver

type DiffDiskDriver interface {
	Driver
	CopyDiff(id, sourceId string) error
}

var (
	// Slice of drivers that should be used in order
	priority = []string{
		"windows",
		"vfs",
	}
)

func GetFSMagic(rootpath string) (FsMagic, error) {
	// Note it is OK to return FsMagicUnsupported on Windows.
	return FsMagicUnsupported, nil
}
