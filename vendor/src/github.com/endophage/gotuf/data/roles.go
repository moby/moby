package data

import (
	"fmt"
	"strings"

	"github.com/endophage/gotuf/errors"
)

var ValidRoles = map[string]string{
	"root":      "root",
	"targets":   "targets",
	"snapshot":  "snapshot",
	"timestamp": "timestamp",
}

func SetValidRoles(rs map[string]string) {
	for k, v := range rs {
		ValidRoles[strings.ToLower(k)] = strings.ToLower(v)
	}
}

func RoleName(role string) string {
	if r, ok := ValidRoles[role]; ok {
		return r
	}
	return role
}

// ValidRole only determines the name is semantically
// correct. For target delegated roles, it does NOT check
// the the appropriate parent roles exist.
func ValidRole(name string) bool {
	name = strings.ToLower(name)
	if v, ok := ValidRoles[name]; ok {
		return name == v
	}
	targetsBase := fmt.Sprintf("%s/", ValidRoles["targets"])
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
	targetsBase := fmt.Sprintf("%s/", ValidRoles["targets"])
	return strings.HasPrefix(r.Name, targetsBase)
}
