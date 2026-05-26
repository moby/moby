//go:build !linux && !windows

package snapshotter

func isMounted(_ string) bool {
	return false
}

func unmount(_ string) error {
	return nil
}
