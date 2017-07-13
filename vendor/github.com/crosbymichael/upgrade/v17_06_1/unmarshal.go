package v17_06_1

import (
	"bytes"
	"encoding/json"
	"fmt"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type linuxSyscalls []linuxSyscall

type linuxSyscall struct {
	specs.LinuxSyscall
}

func (ls *linuxSyscall) UnmarshalJSON(b []byte) error {
	var t struct {
		specs.LinuxSyscall
		Name *string `json:"name,omitempty"`
	}
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	ls.LinuxSyscall = t.LinuxSyscall
	if t.Name != nil {
		if ls.LinuxSyscall.Names != nil {
			return fmt.Errorf("found incompatible 'name' and 'names' fields")
		}
		ls.LinuxSyscall.Names = []string{*t.Name}
		t.Name = nil
	}
	return nil
}

// TODO: figure out how to omitempty when pointer is nil
type memorySwappiness struct {
	V *uint64 `json:",omitempty"`
}

func (m memorySwappiness) String() string {
	if m.V == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%d", *m.V)
}

var null = []byte("null")

func (m *memorySwappiness) MarshalJSON() ([]byte, error) {
	if m.V == nil {
		return null, nil
	}
	return []byte(fmt.Sprintf("%d", *m.V)), nil
}

func (m *memorySwappiness) UnmarshalJSON(b []byte) error {
	if bytes.Compare(b, null) == 0 {
		return nil
	}

	var n uint64
	var i int64
	err := json.Unmarshal(b, &i)
	switch err.(type) {
	case nil:
		n = uint64(i)
	case *json.UnmarshalTypeError:
		// The only valid reason for accepting a uint64 that does not fit into an int64
		// is for erroneous -1 values being converted to uint64.
		// Nevertheless, try unmarshaling it and error out if it's not a number at all.
		if err := json.Unmarshal(b, &n); err != nil {
			return err
		}
	default:
		return err
	}
	if n >= 0 && n <= 100 {
		m.V = &n
	} else {
		m.V = nil
	}
	return nil
}

type linuxCapabilities struct {
	V *specs.LinuxCapabilities
}

func (l *linuxCapabilities) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.V)
}

func (l *linuxCapabilities) UnmarshalJSON(b []byte) error {
	if bytes.Compare(b, null) == 0 {
		return nil
	}
	var s specs.LinuxCapabilities
	err := json.Unmarshal(b, &s)
	switch err.(type) {
	case nil:
		l.V = &s
	case *json.UnmarshalTypeError:
		var caps []string
		err = json.Unmarshal(b, &caps)
		if err != nil {
			return err
		}
		// TODO: copy caps or not copy caps?
		l.V = &specs.LinuxCapabilities{
			Bounding:    caps,
			Effective:   caps,
			Inheritable: caps,
			Permitted:   caps,
			Ambient:     nil,
		}
	}
	return err
}
