package types

import (
	"net"
	"strconv"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestErrorConstructors(t *testing.T) {
	var err error
	var ok bool

	err = InvalidParameterErrorf("Io ho %d uccello", 1)
	assert.Check(t, is.Error(err, "Io ho 1 uccello"))
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	_, ok = err.(MaskableError)
	assert.Check(t, !ok, "error should not be maskable: %[1]v (%[1]T)", err)

	err = NotFoundErrorf("Can't find the %s", "keys")
	assert.Check(t, is.Error(err, "Can't find the keys"))
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	_, ok = err.(MaskableError)
	assert.Check(t, !ok, "error should not be maskable: %[1]v (%[1]T)", err)

	err = ForbiddenErrorf("Can't open door %d", 2)
	assert.Check(t, is.Error(err, "Can't open door 2"))
	assert.Check(t, is.ErrorType(err, cerrdefs.IsPermissionDenied))
	_, ok = err.(MaskableError)
	assert.Check(t, !ok, "error should not be maskable: %[1]v (%[1]T)", err)

	err = NotImplementedErrorf("Functionality %s is not implemented", "x")
	assert.Check(t, is.Error(err, "Functionality x is not implemented"))
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotImplemented))
	_, ok = err.(MaskableError)
	assert.Check(t, !ok, "error should not be maskable: %[1]v (%[1]T)", err)

	err = UnavailableErrorf("Driver %s is not available", "mh")
	assert.Check(t, is.Error(err, "Driver mh is not available"))
	assert.Check(t, is.ErrorType(err, cerrdefs.IsUnavailable))
	_, ok = err.(MaskableError)
	assert.Check(t, !ok, "error should not be maskable: %[1]v (%[1]T)", err)

	err = InternalErrorf("Not sure what happened")
	assert.Check(t, is.Error(err, "Not sure what happened"))
	_, ok = err.(InternalError)
	assert.Check(t, ok, "error should be InternalError: %[1]v (%[1]T)", err)
	_, ok = err.(MaskableError)
	assert.Check(t, !ok, "error should not be maskable: %[1]v (%[1]T)", err)

	err = InternalMaskableErrorf("Minor issue, it can be ignored")
	assert.Check(t, is.Error(err, "Minor issue, it can be ignored"))
	_, ok = err.(InternalError)
	assert.Check(t, ok, "error should be InternalError: %[1]v (%[1]T)", err)
	_, ok = err.(MaskableError)
	assert.Check(t, ok, "error should be maskable: %[1]v (%[1]T)", err)
}

func TestCompareIPMask(t *testing.T) {
	tests := []struct {
		ip    net.IP
		mask  net.IPMask
		is    int
		ms    int
		isErr bool
	}{
		{ // ip in v4Inv6 representation, mask in v4 representation
			ip:   net.IPv4(172, 28, 30, 1),
			mask: []byte{0xff, 0xff, 0xff, 0},
			is:   12,
			ms:   0,
		},
		{ // ip and mask in v4Inv6 representation
			ip:   net.IPv4(172, 28, 30, 2),
			mask: []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0},
			is:   12,
			ms:   12,
		},
		{ // ip in v4 representation, mask in v4Inv6 representation
			ip:   net.IPv4(172, 28, 30, 3)[12:],
			mask: []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0},
			is:   0,
			ms:   12,
		},
		{ // ip and mask in v4 representation
			ip:   net.IPv4(172, 28, 30, 4)[12:],
			mask: []byte{0xff, 0xff, 0xff, 0},
			is:   0,
			ms:   0,
		},
		{ // ip and mask as v6
			ip:   net.ParseIP("2001:DB8:2002:2001:FFFF:ABCD:EEAB:00CD"),
			mask: []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0, 0},
			is:   0,
			ms:   0,
		},
		{
			ip:    net.ParseIP("2001:DB8:2002:2001:FFFF:ABCD:EEAB:00CD"),
			mask:  []byte{0xff, 0xff, 0xff, 0},
			isErr: true,
		},
		{
			ip:    net.ParseIP("173.32.4.5"),
			mask:  []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0},
			isErr: true,
		},
		{
			ip:    net.ParseIP("173.32.4.5"),
			mask:  []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff, 0xff, 0},
			isErr: true,
		},
	}

	for i, tc := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actualIs, actualMs, err := compareIPMask(tc.ip, tc.mask)
			if tc.isErr {
				const expectedErr = "ip and mask are not compatible"
				assert.Check(t, is.ErrorContains(err, expectedErr))
			} else {
				assert.NilError(t, err)
				assert.Check(t, is.Equal(actualIs, tc.is))
				assert.Check(t, is.Equal(actualMs, tc.ms))
			}
		})
	}
}

func TestGetHostPartIP(t *testing.T) {
	tests := []struct {
		ip   net.IP
		mask net.IPMask
		host net.IP
		err  error
	}{
		{ // ip in v4Inv6 representation, mask in v4 representation
			ip:   net.IPv4(172, 28, 30, 1),
			mask: []byte{0xff, 0xff, 0xff, 0},
			host: net.IPv4(0, 0, 0, 1),
		},
		{ // ip and mask in v4Inv6 representation
			ip:   net.IPv4(172, 28, 30, 2),
			mask: []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0},
			host: net.IPv4(0, 0, 0, 2),
		},
		{ // ip in v4 representation, mask in v4Inv6 representation
			ip:   net.IPv4(172, 28, 30, 3)[12:],
			mask: []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0},
			host: net.IPv4(0, 0, 0, 3)[12:],
		},
		{ // ip and mask in v4 representation
			ip:   net.IPv4(172, 28, 30, 4)[12:],
			mask: []byte{0xff, 0xff, 0xff, 0},
			host: net.IPv4(0, 0, 0, 4)[12:],
		},
		{ // ip and mask as v6
			ip:   net.ParseIP("2001:DB8:2002:2001:FFFF:ABCD:EEAB:00CD"),
			mask: []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0, 0},
			host: net.ParseIP("0::AB:00CD"),
		},
	}

	for _, tc := range tests {
		h, err := GetHostPartIP(tc.ip, tc.mask)
		assert.NilError(t, err)
		assert.Assert(t, tc.host.Equal(h), "Failed to return expected host ip. Expected: %s. Got: %s", tc.host, h)
	}

	const expectedErr = "cannot compute host portion ip address because ip and mask are not compatible"

	// ip as v6 and mask as v4 are not compatible
	_, err := GetHostPartIP(net.ParseIP("2001:DB8:2002:2001:FFFF:ABCD:EEAB:00CD"), []byte{0xff, 0xff, 0xff, 0})
	assert.Check(t, is.ErrorContains(err, expectedErr))

	// ip as v4 and non conventional mask
	_, err = GetHostPartIP(net.ParseIP("173.32.4.5"), []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0})
	assert.Check(t, is.ErrorContains(err, expectedErr))

	// ip as v4 and non conventional mask
	_, err = GetHostPartIP(net.ParseIP("173.32.4.5"), []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff, 0xff, 0})
	assert.Check(t, is.ErrorContains(err, expectedErr))
}

func TestGetBroadcastIP(t *testing.T) {
	tests := []struct {
		ip    net.IP
		mask  net.IPMask
		bcast net.IP
		err   error
	}{
		// ip in v4Inv6 representation, mask in v4 representation
		{
			ip:    net.IPv4(172, 28, 30, 1),
			mask:  []byte{0xff, 0xff, 0xff, 0},
			bcast: net.IPv4(172, 28, 30, 255),
		},
		{
			ip:    net.IPv4(10, 28, 30, 1),
			mask:  []byte{0xff, 0, 0, 0},
			bcast: net.IPv4(10, 255, 255, 255),
		},
		// ip and mask in v4Inv6 representation
		{
			ip:    net.IPv4(172, 28, 30, 2),
			mask:  []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0},
			bcast: net.IPv4(172, 28, 30, 255),
		},
		{
			ip:    net.IPv4(172, 28, 30, 2),
			mask:  []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0},
			bcast: net.IPv4(172, 28, 255, 255),
		},
		// ip in v4 representation, mask in v4Inv6 representation
		{
			ip:    net.IPv4(172, 28, 30, 3)[12:],
			mask:  []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0},
			bcast: net.IPv4(172, 28, 30, 255)[12:],
		},
		{
			ip:    net.IPv4(172, 28, 30, 3)[12:],
			mask:  []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0, 0},
			bcast: net.IPv4(172, 255, 255, 255)[12:],
		},
		// ip and mask in v4 representation
		{
			ip:    net.IPv4(172, 28, 30, 4)[12:],
			mask:  []byte{0xff, 0xff, 0xff, 0},
			bcast: net.IPv4(172, 28, 30, 255)[12:],
		},
		{
			ip:    net.IPv4(172, 28, 30, 4)[12:],
			mask:  []byte{0xff, 0xff, 0, 0},
			bcast: net.IPv4(172, 28, 255, 255)[12:],
		},
		{ // ip and mask as v6
			ip:    net.ParseIP("2001:DB8:2002:2001:FFFF:ABCD:EEAB:00CD"),
			mask:  []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0, 0},
			bcast: net.ParseIP("2001:DB8:2002:2001:FFFF:ABCD:EEFF:FFFF"),
		},
	}

	for _, tc := range tests {
		h, err := GetBroadcastIP(tc.ip, tc.mask)
		assert.NilError(t, err)
		assert.Assert(t, tc.bcast.Equal(h), "Failed to return expected host ip. Expected: %s. Got: %s", tc.bcast, h)
	}

	const expectedErr = "cannot compute broadcast ip address because ip and mask are not compatible"

	// ip as v6 and mask as v4 are not compatible
	_, err := GetBroadcastIP(net.ParseIP("2001:DB8:2002:2001:FFFF:ABCD:EEAB:00CD"), []byte{0xff, 0xff, 0xff, 0})
	assert.Check(t, is.ErrorContains(err, expectedErr))

	// ip as v4 and non conventional mask
	_, err = GetBroadcastIP(net.ParseIP("173.32.4.5"), []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0})
	assert.Check(t, is.ErrorContains(err, expectedErr))

	// ip as v4 and non conventional mask
	_, err = GetBroadcastIP(net.ParseIP("173.32.4.5"), []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff, 0xff, 0})
	assert.Check(t, is.ErrorContains(err, expectedErr))
}

func TestParseCIDR(t *testing.T) {
	input := []struct {
		cidr string
		ipnw *net.IPNet
	}{
		{"192.168.22.44/16", &net.IPNet{IP: net.IP{192, 168, 22, 44}, Mask: net.IPMask{255, 255, 0, 0}}},
		{"10.10.2.0/24", &net.IPNet{IP: net.IP{10, 10, 2, 0}, Mask: net.IPMask{255, 255, 255, 0}}},
		{"10.0.0.100/17", &net.IPNet{IP: net.IP{10, 0, 0, 100}, Mask: net.IPMask{255, 255, 128, 0}}},
	}

	for _, i := range input {
		nw, err := ParseCIDR(i.cidr)
		if err != nil {
			t.Fatal(err)
		}
		if !CompareIPNet(nw, i.ipnw) {
			t.Fatalf("network differ. Expected %v. Got: %v", i.ipnw, nw)
		}
	}
}
