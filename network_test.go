package docker

import (
	"net"
	"testing"
)

func TestNetworkRange(t *testing.T) {
	// Simple class C test
	_, network, _ := net.ParseCIDR("192.168.0.1/24")
	first, last := networkRange(network)
	if !first.Equal(net.ParseIP("192.168.0.0")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("192.168.0.255")) {
		t.Error(last.String())
	}
	if size, err := networkSize(network.Mask); err != nil || size != 256 {
		t.Error(size, err)
	}

	// Class A test
	_, network, _ = net.ParseCIDR("10.0.0.1/8")
	first, last = networkRange(network)
	if !first.Equal(net.ParseIP("10.0.0.0")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.255.255.255")) {
		t.Error(last.String())
	}
	if size, err := networkSize(network.Mask); err != nil || size != 16777216 {
		t.Error(size, err)
	}

	// Class A, random IP address
	_, network, _ = net.ParseCIDR("10.1.2.3/8")
	first, last = networkRange(network)
	if !first.Equal(net.ParseIP("10.0.0.0")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.255.255.255")) {
		t.Error(last.String())
	}

	// 32bit mask
	_, network, _ = net.ParseCIDR("10.1.2.3/32")
	first, last = networkRange(network)
	if !first.Equal(net.ParseIP("10.1.2.3")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.1.2.3")) {
		t.Error(last.String())
	}
	if size, err := networkSize(network.Mask); err != nil || size != 1 {
		t.Error(size, err)
	}

	// 31bit mask
	_, network, _ = net.ParseCIDR("10.1.2.3/31")
	first, last = networkRange(network)
	if !first.Equal(net.ParseIP("10.1.2.2")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.1.2.3")) {
		t.Error(last.String())
	}
	if size, err := networkSize(network.Mask); err != nil || size != 2 {
		t.Error(size, err)
	}

	// 26bit mask
	_, network, _ = net.ParseCIDR("10.1.2.3/26")
	first, last = networkRange(network)
	if !first.Equal(net.ParseIP("10.1.2.0")) {
		t.Error(first.String())
	}
	if !last.Equal(net.ParseIP("10.1.2.63")) {
		t.Error(last.String())
	}
	if size, err := networkSize(network.Mask); err != nil || size != 64 {
		t.Error(size, err)
	}
}

func TestConversion(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")
	i, err := ipToInt(ip)
	if err != nil {
		t.Fatal(err)
	}
	if i == 0 {
		t.Fatal("converted to zero")
	}
	conv, err := intToIp(i)
	if err != nil {
		t.Fatal(err)
	}
	if !ip.Equal(conv) {
		t.Error(conv.String())
	}
}

func TestIPAllocator(t *testing.T) {
	gwIP, n, _ := net.ParseCIDR("127.0.0.1/29")
	alloc, err := newIPAllocator(&net.IPNet{gwIP, n.Mask})
	if err != nil {
		t.Fatal(err)
	}
	var lastIP net.IP
	for i := 0; i < 5; i++ {
		ip, err := alloc.Acquire()
		if err != nil {
			t.Fatal(err)
		}
		lastIP = ip
	}
	ip, err := alloc.Acquire()
	if err == nil {
		t.Fatal("There shouldn't be any IP addresses at this point")
	}
	// Release 1 IP
	alloc.Release(lastIP)
	ip, err = alloc.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if !ip.Equal(lastIP) {
		t.Fatal(ip.String())
	}
}
