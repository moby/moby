package network_test

import (
	"encoding/json"
	"testing"

	"github.com/moby/moby/api/types/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

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
		if (c.err == "") != (err == nil) {
			t.Errorf("UnmarshalText(%q) error = %v, want %v", c.in, err, c.err)
		}
		assert.Check(t, is.DeepEqual(a, c.out), "UnmarshalText(%q)", c.in)
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
		assert.Check(t, err, "MarshalText(%v)", c.in)
		assert.Check(t, is.Equal(string(out), c.out), "MarshalText(%v)", c.in)
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
		assert.Check(t, err, "json.Marshal(network.HardwareAddr(%v))", c.in)
		assert.Check(t, is.Equal(string(got), c.out), "json.Marshal(network.HardwareAddr(%v))", c.in)
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
		if (c.err == "") != (err == nil) {
			t.Errorf("json.Unmarshal(%q) error = %v, want %v", c.in, err, c.err)
		}
		assert.Check(t, is.DeepEqual(s.Mac, c.out), "json.Unmarshal(%q)", c.in)
	}
}
