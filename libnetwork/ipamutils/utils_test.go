package ipamutils

import (
	"net"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func initBroadPredefinedNetworks() []*net.IPNet {
	pl := make([]*net.IPNet, 0, 257)
	mask16 := []byte{255, 255, 0, 0}
	for i := 17; i < 18; i++ {
		pl = append(pl, &net.IPNet{IP: []byte{172, byte(i), 0, 0}, Mask: mask16})
	}
	mask24 := []byte{255, 255, 255, 0}
	for i := 0; i < 256; i++ {
		pl = append(pl, &net.IPNet{IP: []byte{172, 18, byte(i), 0}, Mask: mask24})
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

func initGlobalScopeNetworks() []*net.IPNet {
	pl := make([]*net.IPNet, 0, 256*256)
	mask := []byte{255, 255, 255, 0}
	for i := 0; i < 256; i++ {
		for j := 0; j < 256; j++ {
			pl = append(pl, &net.IPNet{IP: []byte{30, byte(i), byte(j), 0}, Mask: mask})
		}
	}
	return pl
}

func TestDefaultNetwork(t *testing.T) {
	for _, nw := range GetGlobalScopeDefaultNetworks() {
		if ones, bits := nw.Mask.Size(); bits != 32 || ones != 24 {
			t.Fatalf("Unexpected size for network in granular list: %v", nw)
		}
	}

	for _, nw := range GetLocalScopeDefaultNetworks() {
		if ones, bits := nw.Mask.Size(); bits != 32 || (ones != 24 && ones != 16) {
			t.Fatalf("Unexpected size for network in broad list: %v", nw)
		}
	}

	originalBroadNets := initBroadPredefinedNetworks()
	m := make(map[string]bool)
	for _, v := range originalBroadNets {
		m[v.String()] = true
	}
	for _, nw := range GetLocalScopeDefaultNetworks() {
		_, ok := m[nw.String()]
		assert.Check(t, ok)
		delete(m, nw.String())
	}

	assert.Check(t, is.Len(m, 0))

	originalGranularNets := initGranularPredefinedNetworks()

	m = make(map[string]bool)
	for _, v := range originalGranularNets {
		m[v.String()] = true
	}
	for _, nw := range GetGlobalScopeDefaultNetworks() {
		_, ok := m[nw.String()]
		assert.Check(t, ok)
		delete(m, nw.String())
	}

	assert.Check(t, is.Len(m, 0))
}

func TestConfigGlobalScopeDefaultNetworks(t *testing.T) {
	err := ConfigGlobalScopeDefaultNetworks([]*NetworkToSplit{{"30.0.0.0/8", 24}})
	assert.NilError(t, err)

	originalGlobalScopeNetworks := initGlobalScopeNetworks()
	m := make(map[string]bool)
	for _, v := range originalGlobalScopeNetworks {
		m[v.String()] = true
	}
	for _, nw := range GetGlobalScopeDefaultNetworks() {
		_, ok := m[nw.String()]
		assert.Check(t, ok)
		delete(m, nw.String())
	}

	assert.Check(t, is.Len(m, 0))
}
