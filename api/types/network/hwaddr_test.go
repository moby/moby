package network_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/moby/moby/api/types/network"
)

func equalHardwareAddr(a, b network.HardwareAddr) bool {
	return (a == nil) == (b == nil) && slices.Equal(a, b)
}

func TestHardwareAddr_UnmarshalText(t *testing.T) {
	cases := []struct {
		in  string
		out network.HardwareAddr
		err string
	}{
		{"", nil, ""},
		{"00:11:22:33:44:55", network.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, ""},
		{"xx:xx:xx:xx:xx:xx", network.HardwareAddr{0xde, 0xad, 0xbe, 0xef}, "invalid MAC address"},
	}
	for _, c := range cases {
		a := network.HardwareAddr{0xde, 0xad, 0xbe, 0xef}
		err := a.UnmarshalText([]byte(c.in))
		if c.err != "" {
			assertErrorContains(t, err, c.err)
		} else {
			assertNoError(t, err)
		}
		if !equalHardwareAddr(a, c.out) {
			t.Errorf("UnmarshalText(%q) = %v, want %v", c.in, a, c.out)
		}
	}
}

func TestHardwareAddr_MarshalText(t *testing.T) {
	cases := []struct {
		in  network.HardwareAddr
		out string
	}{
		{nil, ""},
		{network.HardwareAddr{}, ""},
		{network.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, "00:11:22:33:44:55"},
	}
	for _, c := range cases {
		out, err := c.in.MarshalText()
		assertNoError(t, err)
		assertEqual(t, string(out), c.out)
	}
}

func TestHardwareAddr_MarshalJSON(t *testing.T) {
	cases := []struct {
		in  network.HardwareAddr
		out string
	}{
		{nil, `{"mac":""}`},
		{network.HardwareAddr{}, `{"mac":""}`},
		{network.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, `{"mac":"00:11:22:33:44:55"}`},
	}
	for _, c := range cases {
		s := struct {
			Mac network.HardwareAddr `json:"mac"`
		}{c.in}
		got, err := json.Marshal(s)
		assertNoError(t, err)
		assertEqual(t, string(got), c.out)
	}
}

func TestHardwareAddr_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		in  string
		out network.HardwareAddr
		err string
	}{
		{`{"mac":""}`, nil, ""},
		{`{"mac":"00:11:22:33:44:55"}`, network.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, ""},
		{`{"mac":"xx:xx:xx:xx:xx:xx"}`, network.HardwareAddr{0xde, 0xad, 0xbe, 0xef}, "invalid MAC address"},
	}
	for _, c := range cases {
		s := struct {
			Mac network.HardwareAddr `json:"mac"`
		}{network.HardwareAddr{0xde, 0xad, 0xbe, 0xef}}
		err := json.Unmarshal([]byte(c.in), &s)
		if c.err != "" {
			assertErrorContains(t, err, c.err)
		} else {
			assertNoError(t, err)
		}
		if !equalHardwareAddr(s.Mac, c.out) {
			t.Errorf("json.Unmarshal(%q) = %v, want %v", c.in, s.Mac, c.out)
		}
	}
}
