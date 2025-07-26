package types

import (
	"sort"
)

// PluginsListResponse contains the response for the Engine API
type PluginsListResponse []*Plugin

// PluginPrivilege describes a permission the user has to accept
// upon installing a plugin.
type PluginPrivilege struct {
	Name        string
	Description string
	Value       []string
}

// PluginPrivileges is a list of PluginPrivilege
type PluginPrivileges []PluginPrivilege

func (s PluginPrivileges) Len() int {
	return len(s)
}

func (s PluginPrivileges) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func (s PluginPrivileges) Swap(i, j int) {
	sort.Strings(s[i].Value)
	sort.Strings(s[j].Value)
	s[i], s[j] = s[j], s[i]
}
