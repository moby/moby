//go:build !linux

package libnetwork

import (
	"github.com/docker/docker/libnetwork/types"
)

func (ep *Endpoint) Statistics() (*types.InterfaceStatistics, error) {
	// not implemented on Windows (Endpoint.osIface is always nil)
	return nil, nil
}
