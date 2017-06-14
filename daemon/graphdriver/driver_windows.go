package graphdriver

import "github.com/Microsoft/hcsshim"

var (
	// Slice of drivers that should be used in order
	priority = []string{
		"windowsfilter",
	}
)

// GetFSMagic returns the filesystem id given the path.
func GetFSMagic(rootpath string) (FsMagic, error) {
	// Note it is OK to return FsMagicUnsupported on Windows.
	return FsMagicUnsupported, nil
}

// ApplyDiffOpts contain optional arguments for ApplyDiff()
type ApplyDiffOpts struct {
	// Uvm is the Utility VM where operations are performed
	Uvm hcsshim.Container
}
