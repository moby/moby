package network

import (
	"encoding"
	"fmt"
	"net"
)

// A HardwareAddr represents a physical hardware address.
// It implements [encoding.TextMarshaler] and [encoding.TextUnmarshaler]
// in the absence of go.dev/issue/29678.
type HardwareAddr net.HardwareAddr

var _ encoding.TextMarshaler = (HardwareAddr)(nil)
var _ encoding.TextUnmarshaler = (*HardwareAddr)(nil)
var _ fmt.Stringer = (HardwareAddr)(nil)

func (m *HardwareAddr) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*m = nil
		return nil
	}
	hw, err := net.ParseMAC(string(text))
	if err != nil {
		return err
	}
	*m = HardwareAddr(hw)
	return nil
}

func (m HardwareAddr) MarshalText() ([]byte, error) {
	return []byte(net.HardwareAddr(m).String()), nil
}

func (m HardwareAddr) String() string {
	return net.HardwareAddr(m).String()
}
