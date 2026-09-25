//go:build linux

package overlay

import (
	"encoding/binary"
	"hash/fnv"
	"net"
	"net/netip"
	"testing"
)

func legacyBuildSPI(src, dst net.IP, st uint32) int {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, st)
	h := fnv.New32a()
	h.Write(src)
	h.Write(b)
	h.Write(dst)
	return int(binary.BigEndian.Uint32(h.Sum(nil)))
}

func TestBuildSPI(t *testing.T) {
	cases := []struct {
		src, dst string
		st       uint32
	}{
		{"1.2.3.4", "5.6.7.8", 1234},
		{"::ffff:1.2.3.4", "::ffff:5.6.7.8", 1234},
		{"10.0.0.1", "2001:db8::1", 5678},
		{"2002::abcd:1", "172.15.14.13", 54321},
		{"2002:db8::42", "2002:db8::69", 9999},
	}
	for _, tc := range cases {
		// The legacy buildSPI function is sensitive to whether src and
		// dst are in 4-byte or 16-byte form. Versions of the driver
		// using this function always pass the results of net.ParseIP(),
		// which always parses the address into 16-byte form.
		expected := legacyBuildSPI(net.ParseIP(tc.src), net.ParseIP(tc.dst), tc.st)

		src, dst := netip.MustParseAddr(tc.src), netip.MustParseAddr(tc.dst)
		actual := buildSPI(src, dst, tc.st)
		if expected != actual {
			t.Errorf("buildSPI(%v, %v, %v) = %v; want %v", src, dst, tc.st, actual, expected)
		}
		actual = buildSPI(src.Unmap(), dst.Unmap(), tc.st)
		if expected != actual {
			t.Errorf("buildSPI(%v, %v, %v) = %v; want %v", src.Unmap(), dst.Unmap(), tc.st, actual, expected)
		}
	}
}

func TestMaxMTU(t *testing.T) {
	cases := []struct {
		name     string
		advAddr  string
		netMTU   int
		secure   bool
		expected int
	}{
		{
			name:     "default ipv4 unencrypted",
			advAddr:  "192.168.1.1",
			netMTU:   0,
			secure:   false,
			expected: 1450,
		},
		{
			name:     "default ipv4 encrypted",
			advAddr:  "192.168.1.1",
			netMTU:   0,
			secure:   true,
			expected: 1424,
		},
		{
			name:     "default ipv6 unencrypted",
			advAddr:  "2001:db8::1",
			netMTU:   0,
			secure:   false,
			expected: 1430,
		},
		{
			name:     "default ipv6 encrypted",
			advAddr:  "2001:db8::1",
			netMTU:   0,
			secure:   true,
			expected: 1404,
		},
		{
			name:     "custom mtu ipv6 unencrypted",
			advAddr:  "2001:db8::1",
			netMTU:   9000,
			secure:   false,
			expected: 8930,
		},
		{
			name:     "custom mtu ipv6 encrypted",
			advAddr:  "2001:db8::1",
			netMTU:   9000,
			secure:   true,
			expected: 8904,
		},
		{
			name:     "unknown transport defaults to ipv4 encap",
			advAddr:  "",
			netMTU:   0,
			secure:   false,
			expected: 1450,
		},
		{
			name:     "nil driver defaults to ipv4 encap",
			advAddr:  "nil",
			netMTU:   0,
			secure:   false,
			expected: 1450,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var d *driver
			if tc.advAddr != "nil" {
				d = &driver{}
				if tc.advAddr != "" {
					d.advertiseAddress = netip.MustParseAddr(tc.advAddr)
				}
			}
			n := &network{
				driver: d,
				mtu:    tc.netMTU,
				secure: tc.secure,
			}
			if got := n.maxMTU(); got != tc.expected {
				t.Errorf("maxMTU() = %d, want %d", got, tc.expected)
			}
		})
	}
}
