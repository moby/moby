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
func (s Privileges) Len() int {
	return len(s)
}

// Less implements [sort.Interface].
func (s Privileges) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

// Swap implements [sort.Interface].
func (s Privileges) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
