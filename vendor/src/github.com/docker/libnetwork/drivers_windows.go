package libnetwork

import (
	"github.com/docker/libnetwork/drivers/null"
	"github.com/docker/libnetwork/drivers/windows"
)

func getInitializers() []initializer {
	return []initializer{
		{null.Init, "null"},
		{windows.GetInit("Transparent"), "Transparent"},
		{windows.GetInit("L2Bridge"), "L2Bridge"},
		{windows.GetInit("L2Tunnel"), "L2Tunnel"},
		{windows.GetInit("NAT"), "NAT"},
	}
}
