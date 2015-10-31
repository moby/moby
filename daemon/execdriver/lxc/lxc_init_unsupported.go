// +build !linux

package lxc

// InitArgs contains args provided to the init function for a driver
type InitArgs struct {
}

func finalizeNamespace(args *InitArgs) error {
	panic("Not supported on this platform")
}
