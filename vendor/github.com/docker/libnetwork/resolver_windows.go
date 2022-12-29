//go:build windows
// +build windows

package libnetwork

func (r *resolver) setupIPTable() error {
	return nil
}
