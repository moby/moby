package ipamutils

import (
	"net"
	"net/netip"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
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
	globalNetworks := GetDefaultGlobalScopeSubnetter()
	for nw, err := globalNetworks.NextSubnet(); err == nil; nw, err = globalNetworks.NextSubnet() {
		if bits := nw.Bits(); bits != 24 {
			t.Fatalf("Unexpected size for network in global list: %v", nw)
		}
	}

	localNetworks := GetDefaultLocalScopeSubnetter()
	for nw, err := localNetworks.NextSubnet(); err == nil; nw, err = localNetworks.NextSubnet() {
		if bits := nw.Bits(); bits != 20 && bits != 16 {
			t.Fatalf("Unexpected size for network in local list: %v", nw)
		}
	}

	originalBroadNets := initBroadPredefinedNetworks()
	m := make(map[string]bool)
	for _, v := range originalBroadNets {
		m[v.String()] = true
	}
	localNetworks = GetDefaultLocalScopeSubnetter()
	for nw, err := localNetworks.NextSubnet(); err == nil; nw, err = localNetworks.NextSubnet() {
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
	globalNetworks = GetDefaultGlobalScopeSubnetter()
	for nw, err := globalNetworks.NextSubnet(); err == nil; nw, err = globalNetworks.NextSubnet() {
		_, ok := m[nw.String()]
		assert.Check(t, ok)
		delete(m, nw.String())
	}

	assert.Check(t, is.Len(m, 0))
}

func TestConfigGlobalScopeDefaultNetworks(t *testing.T) {
	err := ConfigGlobalScopeDefaultNetworks([]*NetworkToSplit{{Base: "30.0.0.0/8", Size: 24}})
	assert.NilError(t, err)

	originalGlobalScopeNetworks := initGlobalScopeNetworks()
	m := make(map[string]bool)
	for _, v := range originalGlobalScopeNetworks {
		m[v.String()] = true
	}
	globalNetworks := GetDefaultGlobalScopeSubnetter()
	for nw, err := globalNetworks.NextSubnet(); err == nil; nw, err = globalNetworks.NextSubnet() {
		str := nw.String()
		_, ok := m[str]
		assert.Check(t, ok)
		delete(m, str)
	}

	assert.Check(t, is.Len(m, 0))
}

func TestSubnetter(t *testing.T) {
	s, err := NewSubnetter([]*NetworkToSplit{
		{Base: "172.80.0.0/16", Size: 24},
		{Base: "172.90.0.0/16", Size: 24}})
	assert.NilError(t, err)

	var nets []netip.Prefix
	for i := 0; i < 512; i++ {
		if nw, err := s.NextSubnet(); err == nil {
			nets = append(nets, nw)
		} else {
			t.Fatal("Subnetter is smaller than expected.")
		}
	}

	assert.Check(t, is.Equal(nets[0].String(), "172.80.0.0/24"))
	assert.Check(t, is.Equal(nets[127].String(), "172.80.127.0/24"))
	assert.Check(t, is.Equal(nets[255].String(), "172.80.255.0/24"))
	assert.Check(t, is.Equal(nets[256].String(), "172.90.0.0/24"))
	assert.Check(t, is.Equal(nets[383].String(), "172.90.127.0/24"))
	assert.Check(t, is.Equal(nets[511].String(), "172.90.255.0/24"))
}

func TestInitPoolWithIPv6(t *testing.T) {
	s, err := NewSubnetter([]*NetworkToSplit{
		{Base: "2001:db8:1:1f00::/64", Size: 96},
		{Base: "fd00::/8", Size: 96},
		{Base: "fd00::/16", Size: 32},
		{Base: "fd00:0:0:fff0::/63", Size: 65},
		{Base: "fd00::/16", Size: 72},
	})
	assert.NilError(t, err)

	// Iterating over the Subnetter would be way too long, so better change its offset and see what
	// NextSubnet() returns.
	s.offset = (1 << 32) - 1
	nw, err := s.NextSubnet()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(nw.String(), "2001:db8:1:1f00:ffff:ffff::/96"))

	s.index = 1
	s.offset = (1 << 64) - 1
	nw, err = s.NextSubnet()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(nw.String(), "fd00:0:ffff:ffff:ffff:ffff::/96"))

	// Previous Get() call incremented index by 1. Next call yields a subnet from the 3rd NetworkToSplit.
	nw, err = s.NextSubnet()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(nw.String(), "fd00::/32"))

	s.index = 2
	s.offset = (1 << 16) - 3
	nw, err = s.NextSubnet()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(nw.String(), "fd00:fffd::/32"))

	s.index = 3
	s.offset = 1
	nw, err = s.NextSubnet()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(nw.String(), "fd00:0:0:fff0:8000::/65"))

	s.index = 3
	s.offset = 2
	nw, err = s.NextSubnet()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(nw.String(), "fd00:0:0:fff1::/65"))

	s.index = 4
	s.offset = 2000
	nw, err = s.NextSubnet()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(nw.String(), "fd00:0:0:7:d000::/72"))

	s.offset = 1<<56 - 1
	nw, err = s.NextSubnet()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(nw.String(), "fd00:ffff:ffff:ffff:ff00::/72"))

	// After the Subnetter yields the last subnet from the last NetworkToSplit, it should then return ErrNoMoreSubnet.
	_, err = s.NextSubnet()
	assert.ErrorIs(t, err, ErrNoMoreSubnet)
}
