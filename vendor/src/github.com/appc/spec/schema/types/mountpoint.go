package types

type MountPoint struct {
	Name     ACName `json:"name"`
	Path     string `json:"path"`
	ReadOnly bool   `json:"readOnly,omitempty"`
}
