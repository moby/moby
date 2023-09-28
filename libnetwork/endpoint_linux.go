package libnetwork

import (
	"fmt"

	"github.com/docker/docker/libnetwork/types"
)

// Statistics returns a pointer to InterfaceStatistics from the underlying interface, or an error if this endpoint does
// not have an underlying interface.
//
// FIXME(neersighted): This is a horrible kludge and relies on ep.osIface being set in sbox.populateNetworkResources().
func (ep *Endpoint) Statistics() (*types.InterfaceStatistics, error) {
	if ep.osIface == nil {
		return nil, fmt.Errorf("invalid osIface in endpoint %s", ep.Name())
	}

	return ep.osIface.Statistics()
}
