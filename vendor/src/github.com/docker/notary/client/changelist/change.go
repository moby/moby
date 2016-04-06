package changelist

import (
	"github.com/docker/notary/tuf/data"
)

// Scopes for TufChanges are simply the TUF roles.
// Unfortunately because of targets delegations, we can only
// cover the base roles.
const (
	ScopeRoot      = "root"
	ScopeTargets   = "targets"
	ScopeSnapshot  = "snapshot"
	ScopeTimestamp = "timestamp"
)

// Types for TufChanges are namespaced by the Role they
// are relevant for. The Root and Targets roles are the
// only ones for which user action can cause a change, as
// all changes in Snapshot and Timestamp are programmatically
// generated base on Root and Targets changes.
const (
	TypeRootRole          = "role"
	TypeTargetsTarget     = "target"
	TypeTargetsDelegation = "delegation"
)

// TufChange represents a change to a TUF repo
type TufChange struct {
	// Abbreviated because Go doesn't permit a field and method of the same name
	Actn       string `json:"action"`
	Role       string `json:"role"`
	ChangeType string `json:"type"`
	ChangePath string `json:"path"`
	Data       []byte `json:"data"`
}

// TufRootData represents a modification of the keys associated
// with a role that appears in the root.json
type TufRootData struct {
	Keys     data.KeyList `json:"keys"`
	RoleName string       `json:"role"`
}

// NewTufChange initializes a tufChange object
func NewTufChange(action string, role, changeType, changePath string, content []byte) *TufChange {
	return &TufChange{
		Actn:       action,
		Role:       role,
		ChangeType: changeType,
		ChangePath: changePath,
		Data:       content,
	}
}

// Action return c.Actn
func (c TufChange) Action() string {
	return c.Actn
}

// Scope returns c.Role
func (c TufChange) Scope() string {
	return c.Role
}

// Type returns c.ChangeType
func (c TufChange) Type() string {
	return c.ChangeType
}

// Path return c.ChangePath
func (c TufChange) Path() string {
	return c.ChangePath
}

// Content returns c.Data
func (c TufChange) Content() []byte {
	return c.Data
}

// TufDelegation represents a modification to a target delegation
// this includes creating a delegations. This format is used to avoid
// unexpected race conditions between humans modifying the same delegation
type TufDelegation struct {
	NewName       string       `json:"new_name,omitempty"`
	NewThreshold  int          `json:"threshold, omitempty"`
	AddKeys       data.KeyList `json:"add_keys, omitempty"`
	RemoveKeys    []string     `json:"remove_keys,omitempty"`
	AddPaths      []string     `json:"add_paths,omitempty"`
	RemovePaths   []string     `json:"remove_paths,omitempty"`
	ClearAllPaths bool         `json:"clear_paths,omitempty"`
}

// ToNewRole creates a fresh role object from the TufDelegation data
func (td TufDelegation) ToNewRole(scope string) (*data.Role, error) {
	name := scope
	if td.NewName != "" {
		name = td.NewName
	}
	return data.NewRole(name, td.NewThreshold, td.AddKeys.IDs(), td.AddPaths)
}
