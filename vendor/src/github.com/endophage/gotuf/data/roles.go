package data

import (
	"fmt"
	"strings"

	"github.com/endophage/gotuf/errors"
)

// Canonical base role names
const (
	CanonicalRootRole      = "root"
	CanonicalTargetsRole   = "targets"
	CanonicalSnapshotRole  = "snapshot"
	CanonicalTimestampRole = "timestamp"
)

var ValidRoles = map[string]string{
	CanonicalRootRole:      CanonicalRootRole,
	CanonicalTargetsRole:   CanonicalTargetsRole,
	CanonicalSnapshotRole:  CanonicalSnapshotRole,
	CanonicalTimestampRole: CanonicalTimestampRole,
}

func SetValidRoles(rs map[string]string) {
	// iterate ValidRoles
	for k, _ := range ValidRoles {
		if v, ok := rs[k]; ok {
			ValidRoles[k] = v
		}
	}
}

func RoleName(role string) string {
	if r, ok := ValidRoles[role]; ok {
		return r
	}
	return role
}

func CanonicalRole(role string) string {
	name := strings.ToLower(role)
	if _, ok := ValidRoles[name]; ok {
		// The canonical version is always lower case
		// se ensure we return name, not role
		return name
	}
	targetsBase := fmt.Sprintf("%s/", ValidRoles[CanonicalTargetsRole])
	if strings.HasPrefix(name, targetsBase) {
		role = strings.TrimPrefix(role, targetsBase)
		role = fmt.Sprintf("%s/%s", CanonicalTargetsRole, role)
		return role
	}
	for r, v := range ValidRoles {
		if role == v {
			return r
		}
	}
	return ""
}

// ValidRole only determines the name is semantically
// correct. For target delegated roles, it does NOT check
// the the appropriate parent roles exist.
func ValidRole(name string) bool {
	name = strings.ToLower(name)
	if v, ok := ValidRoles[name]; ok {
		return name == v
	}
	targetsBase := fmt.Sprintf("%s/", ValidRoles[CanonicalTargetsRole])
	if strings.HasPrefix(name, targetsBase) {
		return true
	}
	for _, v := range ValidRoles {
		if name == v {
			return true
		}
	}
	return false
}

type RootRole struct {
	KeyIDs    []string `json:"keyids"`
	Threshold int      `json:"threshold"`
}
type Role struct {
	RootRole
	Name             string   `json:"name"`
	Paths            []string `json:"paths,omitempty"`
	PathHashPrefixes []string `json:"path_hash_prefixes,omitempty"`
}

func NewRole(name string, threshold int, keyIDs, paths, pathHashPrefixes []string) (*Role, error) {
	if len(paths) > 0 && len(pathHashPrefixes) > 0 {
		return nil, errors.ErrInvalidRole{}
	}
	if threshold < 1 {
		return nil, errors.ErrInvalidRole{}
	}
	if !ValidRole(name) {
		return nil, errors.ErrInvalidRole{}
	}
	return &Role{
		RootRole: RootRole{
			KeyIDs:    keyIDs,
			Threshold: threshold,
		},
		Name:             name,
		Paths:            paths,
		PathHashPrefixes: pathHashPrefixes,
	}, nil

}

func (r Role) IsValid() bool {
	return !(len(r.Paths) > 0 && len(r.PathHashPrefixes) > 0)
}

func (r Role) ValidKey(id string) bool {
	for _, key := range r.KeyIDs {
		if key == id {
			return true
		}
	}
	return false
}

func (r Role) CheckPaths(path string) bool {
	for _, p := range r.Paths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func (r Role) CheckPrefixes(hash string) bool {
	for _, p := range r.PathHashPrefixes {
		if strings.HasPrefix(hash, p) {
			return true
		}
	}
	return false
}

func (r Role) IsDelegation() bool {
	targetsBase := fmt.Sprintf("%s/", ValidRoles[CanonicalTargetsRole])
	return strings.HasPrefix(r.Name, targetsBase)
}
