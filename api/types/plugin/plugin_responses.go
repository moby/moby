package plugin

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ListResponse contains the response for the Engine API
type ListResponse []*Plugin

// UnmarshalJSON implements json.Unmarshaler for PluginInterfaceType
func (t *InterfaceType) UnmarshalJSON(p []byte) error {
	versionIndex := len(p)
	prefixIndex := 0
	if len(p) < 2 || p[0] != '"' || p[len(p)-1] != '"' {
		return fmt.Errorf("%q is not a plugin interface type", p)
	}
	p = p[1 : len(p)-1]
loop:
	for i, b := range p {
		switch b {
		case '.':
			prefixIndex = i
		case '/':
			versionIndex = i
			break loop
		}
	}
	t.Prefix = string(p[:prefixIndex])
	t.Capability = string(p[prefixIndex+1 : versionIndex])
	if versionIndex < len(p) {
		t.Version = string(p[versionIndex+1:])
	}
	return nil
}

// MarshalJSON implements json.Marshaler for PluginInterfaceType
func (t *InterfaceType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// String implements fmt.Stringer for PluginInterfaceType
func (t InterfaceType) String() string {
	return fmt.Sprintf("%s.%s/%s", t.Prefix, t.Capability, t.Version)
}

// Privilege describes a permission the user has to accept
// upon installing a plugin.
type Privilege struct {
	Name        string
	Description string
	Value       []string
}

// Privileges is a list of Privilege
type Privileges []Privilege

func (s Privileges) Len() int {
	return len(s)
}

func (s Privileges) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func (s Privileges) Swap(i, j int) {
	sort.Strings(s[i].Value)
	sort.Strings(s[j].Value)
	s[i], s[j] = s[j], s[i]
}
