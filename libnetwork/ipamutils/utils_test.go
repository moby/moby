package ipamutils

import (
	"net"
	"sync"
	"testing"

	_ "github.com/docker/libnetwork/testutils"
	"github.com/stretchr/testify/assert"
)

func initBroadPredefinedNetworks() []*net.IPNet {
	pl := make([]*net.IPNet, 0, 31)
	mask := []byte{255, 255, 0, 0}
	for i := 17; i < 32; i++ {
		pl = append(pl, &net.IPNet{IP: []byte{172, byte(i), 0, 0}, Mask: mask})
	}
	mask20 := []byte{255, 255, 240, 0}
	for i := 0; i < 16; i++ {
		pl = append(pl, &net.IPNet{IP: []byte{192, 168, byte(i << 4), 0}, Mask: mask20})
	}
	return pl
}

func initGranularPredefinedNetworks() []*net.IPNet {
	pl := make([]*net.IPNet, 0, 256*256)
	mask := []byte{255, 255, 255, 0}
	for i := 0; i < 256; i++ {
		for j := 0; j < 256; j++ {
			pl = append(pl, &net.IPNet{IP: []byte{10, byte(i), byte(j), 0}, Mask: mask})
		}
	}
	return pl
}

func TestDefaultNetwork(t *testing.T) {
	InitNetworks(nil)
	for _, nw := range PredefinedGranularNetworks {
		if ones, bits := nw.Mask.Size(); bits != 32 || ones != 24 {
			t.Fatalf("Unexpected size for network in granular list: %v", nw)
		}
	}

	for _, nw := range PredefinedBroadNetworks {
		if ones, bits := nw.Mask.Size(); bits != 32 || (ones != 20 && ones != 16) {
			t.Fatalf("Unexpected size for network in broad list: %v", nw)
		}
	}

	originalBroadNets := initBroadPredefinedNetworks()
	m := make(map[string]bool)
	for _, v := range originalBroadNets {
		m[v.String()] = true
	}
	for _, nw := range PredefinedBroadNetworks {
		_, ok := m[nw.String()]
		assert.True(t, ok)
		delete(m, nw.String())
	}

	assert.Len(t, m, 0)

	originalGranularNets := initGranularPredefinedNetworks()

	m = make(map[string]bool)
	for _, v := range originalGranularNets {
		m[v.String()] = true
	}
	for _, nw := range PredefinedGranularNetworks {
		_, ok := m[nw.String()]
		assert.True(t, ok)
		delete(m, nw.String())
	}

	assert.Len(t, m, 0)
}

func TestInitAddressPools(t *testing.T) {
	initNetworksOnce = sync.Once{}
	InitNetworks([]*NetworkToSplit{{"172.80.0.0/16", 24}, {"172.90.0.0/16", 24}})

	// Check for Random IPAddresses in PredefinedBroadNetworks  ex: first , last and middle
	assert.Len(t, PredefinedBroadNetworks, 512, "Failed to find PredefinedBroadNetworks")
	assert.Equal(t, PredefinedBroadNetworks[0].String(), "172.80.0.0/24")
	assert.Equal(t, PredefinedBroadNetworks[127].String(), "172.80.127.0/24")
	assert.Equal(t, PredefinedBroadNetworks[255].String(), "172.80.255.0/24")
	assert.Equal(t, PredefinedBroadNetworks[256].String(), "172.90.0.0/24")
	assert.Equal(t, PredefinedBroadNetworks[383].String(), "172.90.127.0/24")
	assert.Equal(t, PredefinedBroadNetworks[511].String(), "172.90.255.0/24")
}
