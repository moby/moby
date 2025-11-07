package plugin

import (
	"sort"
)

// ListResponse contains the response for the Engine API
type ListResponse []Plugin

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
