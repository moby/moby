//go:build windows
// +build windows

package libnetwork

func (r *Resolver) setupIPTable() error {
	return nil
}
