package graphdriver

type DiffDiskDriver interface {
	Driver
	CopyDiff(id, sourceId string) error
}

const (
	FsMagicWindows = FsMagic(0xa1b1830f)
)

var (
	// Slice of drivers that should be used in an order
	priority = []string{
		"windows",
	}

	FsNames = map[FsMagic]string{
		FsMagicWindows:     "windows",
		FsMagicUnsupported: "unsupported",
	}
)

func GetFSMagic(rootpath string) (FsMagic, error) {
	return FsMagicWindows, nil
}
