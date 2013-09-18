// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ipv4_test

import (
	"bytes"
	"code.google.com/p/go.net/ipv4"
	"net"
	"reflect"
	"runtime"
	"testing"
)

var (
	wireHeaderFromKernel = [ipv4.HeaderLen]byte{
		0x45, 0x01, 0xbe, 0xef,
		0xca, 0xfe, 0x05, 0xdc,
		0xff, 0x01, 0xde, 0xad,
		172, 16, 254, 254,
		192, 168, 0, 1,
	}
	wireHeaderToKernel = [ipv4.HeaderLen]byte{
		0x45, 0x01, 0xbe, 0xef,
		0xca, 0xfe, 0x05, 0xdc,
		0xff, 0x01, 0xde, 0xad,
		172, 16, 254, 254,
		192, 168, 0, 1,
	}
	wireHeaderFromTradBSDKernel = [ipv4.HeaderLen]byte{
		0x45, 0x01, 0xdb, 0xbe,
		0xca, 0xfe, 0xdc, 0x05,
		0xff, 0x01, 0xde, 0xad,
		172, 16, 254, 254,
		192, 168, 0, 1,
	}
	wireHeaderToTradBSDKernel = [ipv4.HeaderLen]byte{
		0x45, 0x01, 0xef, 0xbe,
		0xca, 0xfe, 0xdc, 0x05,
		0xff, 0x01, 0xde, 0xad,
		172, 16, 254, 254,
		192, 168, 0, 1,
	}
	// TODO(mikio): Add platform dependent wire header formats when
	// we support new platforms.

	testHeader = &ipv4.Header{
		Version:  ipv4.Version,
		Len:      ipv4.HeaderLen,
		TOS:      1,
		TotalLen: 0xbeef,
		ID:       0xcafe,
		FragOff:  1500,
		TTL:      255,
		Protocol: 1,
		Checksum: 0xdead,
		Src:      net.IPv4(172, 16, 254, 254),
		Dst:      net.IPv4(192, 168, 0, 1),
	}
)

func TestMarshalHeader(t *testing.T) {
	b, err := testHeader.Marshal()
	if err != nil {
		t.Fatalf("ipv4.Header.Marshal failed: %v", err)
	}
	var wh []byte
	switch runtime.GOOS {
	case "linux", "openbsd":
		wh = wireHeaderToKernel[:]
	default:
		wh = wireHeaderToTradBSDKernel[:]
	}
	if !bytes.Equal(b, wh) {
		t.Fatalf("ipv4.Header.Marshal failed: %#v not equal %#v", b, wh)
	}
}

func TestParseHeader(t *testing.T) {
	var wh []byte
	switch runtime.GOOS {
	case "linux", "openbsd":
		wh = wireHeaderFromKernel[:]
	default:
		wh = wireHeaderFromTradBSDKernel[:]
	}
	h, err := ipv4.ParseHeader(wh)
	if err != nil {
		t.Fatalf("ipv4.ParseHeader failed: %v", err)
	}
	if !reflect.DeepEqual(h, testHeader) {
		t.Fatalf("ipv4.ParseHeader failed: %#v not equal %#v", h, testHeader)
	}
}
