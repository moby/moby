//go:build windows

package libnetwork

import "context"

func (r *Resolver) setupNAT(context.Context) error {
	return nil
}
