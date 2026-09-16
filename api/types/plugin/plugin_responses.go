package plugin

// ListResponse contains the response for the Engine API
type ListResponse []Plugin

// Privilege describes a permission the user has to accept
// upon installing a plugin.
type Privilege struct {
	Name        string
	Description string
	Value       []string
}

// Privileges is a list of Privilege.
type Privileges []Privilege

// Len implements [sort.Interface].
//
// Deprecated: use [slices.SortFunc] to sort privileges instead.
func (s Privileges) Len() int {
	return len(s)
}

// Less implements [sort.Interface].
//
// Deprecated: use [slices.SortFunc] to sort privileges instead.
func (s Privileges) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// Swap implements [sort.Interface].
//
// Deprecated: use [slices.SortFunc] to sort privileges instead.
func (s Privileges) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
