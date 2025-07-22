package plugin

import (
	"bytes"
	"encoding"
	"fmt"
	"strings"
)

type CapabilityID struct {
	Capability string
	Prefix     string
	Version    string
}

var (
	_ fmt.Stringer             = CapabilityID{}
	_ encoding.TextUnmarshaler = (*CapabilityID)(nil)
	_ encoding.TextMarshaler   = CapabilityID{}
)

// String implements [fmt.Stringer] for CapabilityID
func (t CapabilityID) String() string {
	return fmt.Sprintf("%s.%s/%s", t.Prefix, t.Capability, t.Version)
}

// UnmarshalText implements [encoding.TextUnmarshaler] for CapabilityID
func (t *CapabilityID) UnmarshalText(p []byte) error {
	fqcap, version, _ := bytes.Cut(p, []byte{'/'})
	idx := bytes.LastIndexByte(fqcap, '.')
	if idx < 0 {
		t.Prefix = ""
		t.Capability = string(fqcap)
	} else {
		t.Prefix = string(fqcap[:idx])
		t.Capability = string(fqcap[idx+1:])
	}
	t.Version = string(version)
	return nil
}

// MarshalText implements [encoding.TextMarshaler] for CapabilityID
func (t CapabilityID) MarshalText() ([]byte, error) {
	// Assert that the value can be round-tripped successfully.
	if strings.Contains(t.Capability, ".") {
		return nil, fmt.Errorf("capability %q cannot contain a dot", t.Capability)
	}
	if strings.Contains(t.Prefix, "/") {
		return nil, fmt.Errorf("prefix %q cannot contain a slash", t.Prefix)
	}
	if strings.Contains(t.Capability, "/") {
		return nil, fmt.Errorf("capability %q cannot contain a slash", t.Capability)
	}
	return []byte(t.String()), nil
}
