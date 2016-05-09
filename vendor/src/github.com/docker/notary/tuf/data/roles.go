package data

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/Sirupsen/logrus"
)

// Canonical base role names
const (
	CanonicalRootRole      = "root"
	CanonicalTargetsRole   = "targets"
	CanonicalSnapshotRole  = "snapshot"
	CanonicalTimestampRole = "timestamp"
)

// BaseRoles is an easy to iterate list of the top level
// roles.
var BaseRoles = []string{
	CanonicalRootRole,
	CanonicalTargetsRole,
	CanonicalSnapshotRole,
	CanonicalTimestampRole,
}

// Regex for validating delegation names
var delegationRegexp = regexp.MustCompile("^[-a-z0-9_/]+$")

// ErrNoSuchRole indicates the roles doesn't exist
type ErrNoSuchRole struct {
	Role string
}

func (e ErrNoSuchRole) Error() string {
	return fmt.Sprintf("role does not exist: %s", e.Role)
}

// ErrInvalidRole represents an error regarding a role. Typically
// something like a role for which sone of the public keys were
// not found in the TUF repo.
type ErrInvalidRole struct {
	Role   string
	Reason string
}

func (e ErrInvalidRole) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("tuf: invalid role %s. %s", e.Role, e.Reason)
	}
	return fmt.Sprintf("tuf: invalid role %s.", e.Role)
}

// ValidRole only determines the name is semantically
// correct. For target delegated roles, it does NOT check
// the the appropriate parent roles exist.
func ValidRole(name string) bool {
	if IsDelegation(name) {
		return true
	}

	for _, v := range BaseRoles {
		if name == v {
			return true
		}
	}
	return false
}

// IsDelegation checks if the role is a delegation or a root role
func IsDelegation(role string) bool {
	targetsBase := CanonicalTargetsRole + "/"

	whitelistedChars := delegationRegexp.MatchString(role)

	// Limit size of full role string to 255 chars for db column size limit
	correctLength := len(role) < 256

	// Removes ., .., extra slashes, and trailing slash
	isClean := path.Clean(role) == role
	return strings.HasPrefix(role, targetsBase) &&
		whitelistedChars &&
		correctLength &&
		isClean
}

// BaseRole is an internal representation of a root/targets/snapshot/timestamp role, with its public keys included
type BaseRole struct {
	Keys      map[string]PublicKey
	Name      string
	Threshold int
}

// NewBaseRole creates a new BaseRole object with the provided parameters
func NewBaseRole(name string, threshold int, keys ...PublicKey) BaseRole {
	r := BaseRole{
		Name:      name,
		Threshold: threshold,
		Keys:      make(map[string]PublicKey),
	}
	for _, k := range keys {
		r.Keys[k.ID()] = k
	}
	return r
}

// ListKeys retrieves the public keys valid for this role
func (b BaseRole) ListKeys() KeyList {
	return listKeys(b.Keys)
}

// ListKeyIDs retrieves the list of key IDs valid for this role
func (b BaseRole) ListKeyIDs() []string {
	return listKeyIDs(b.Keys)
}

// Equals returns whether this BaseRole equals another BaseRole
func (b BaseRole) Equals(o BaseRole) bool {
	if b.Threshold != o.Threshold || b.Name != o.Name || len(b.Keys) != len(o.Keys) {
		return false
	}

	for keyID, key := range b.Keys {
		oKey, ok := o.Keys[keyID]
		if !ok || key.ID() != oKey.ID() {
			return false
		}
	}

	return true
}

// DelegationRole is an internal representation of a delegation role, with its public keys included
type DelegationRole struct {
	BaseRole
	Paths []string
}

func listKeys(keyMap map[string]PublicKey) KeyList {
	keys := KeyList{}
	for _, key := range keyMap {
		keys = append(keys, key)
	}
	return keys
}

func listKeyIDs(keyMap map[string]PublicKey) []string {
	keyIDs := []string{}
	for id := range keyMap {
		keyIDs = append(keyIDs, id)
	}
	return keyIDs
}

// Restrict restricts the paths and path hash prefixes for the passed in delegation role,
// returning a copy of the role with validated paths as if it was a direct child
func (d DelegationRole) Restrict(child DelegationRole) (DelegationRole, error) {
	if !d.IsParentOf(child) {
		return DelegationRole{}, fmt.Errorf("%s is not a parent of %s", d.Name, child.Name)
	}
	return DelegationRole{
		BaseRole: BaseRole{
			Keys:      child.Keys,
			Name:      child.Name,
			Threshold: child.Threshold,
		},
		Paths: RestrictDelegationPathPrefixes(d.Paths, child.Paths),
	}, nil
}

// IsParentOf returns whether the passed in delegation role is the direct child of this role,
// determined by delegation name.
// Ex: targets/a is a direct parent of targets/a/b, but targets/a is not a direct parent of targets/a/b/c
func (d DelegationRole) IsParentOf(child DelegationRole) bool {
	return path.Dir(child.Name) == d.Name
}

// CheckPaths checks if a given path is valid for the role
func (d DelegationRole) CheckPaths(path string) bool {
	return checkPaths(path, d.Paths)
}

func checkPaths(path string, permitted []string) bool {
	for _, p := range permitted {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// RestrictDelegationPathPrefixes returns the list of valid delegationPaths that are prefixed by parentPaths
func RestrictDelegationPathPrefixes(parentPaths, delegationPaths []string) []string {
	validPaths := []string{}
	if len(delegationPaths) == 0 {
		return validPaths
	}

	// Validate each individual delegation path
	for _, delgPath := range delegationPaths {
		isPrefixed := false
		for _, parentPath := range parentPaths {
			if strings.HasPrefix(delgPath, parentPath) {
				isPrefixed = true
				break
			}
		}
		// If the delegation path did not match prefix against any parent path, it is not valid
		if isPrefixed {
			validPaths = append(validPaths, delgPath)
		}
	}
	return validPaths
}

// RootRole is a cut down role as it appears in the root.json
// Eventually should only be used for immediately before and after serialization/deserialization
type RootRole struct {
	KeyIDs    []string `json:"keyids"`
	Threshold int      `json:"threshold"`
}

// Role is a more verbose role as they appear in targets delegations
// Eventually should only be used for immediately before and after serialization/deserialization
type Role struct {
	RootRole
	Name  string   `json:"name"`
	Paths []string `json:"paths,omitempty"`
}

// NewRole creates a new Role object from the given parameters
func NewRole(name string, threshold int, keyIDs, paths []string) (*Role, error) {
	if IsDelegation(name) {
		if len(paths) == 0 {
			logrus.Debugf("role %s with no Paths will never be able to publish content until one or more are added", name)
		}
	}
	if threshold < 1 {
		return nil, ErrInvalidRole{Role: name}
	}
	if !ValidRole(name) {
		return nil, ErrInvalidRole{Role: name}
	}
	return &Role{
		RootRole: RootRole{
			KeyIDs:    keyIDs,
			Threshold: threshold,
		},
		Name:  name,
		Paths: paths,
	}, nil

}

// CheckPaths checks if a given path is valid for the role
func (r Role) CheckPaths(path string) bool {
	return checkPaths(path, r.Paths)
}

// AddKeys merges the ids into the current list of role key ids
func (r *Role) AddKeys(ids []string) {
	r.KeyIDs = mergeStrSlices(r.KeyIDs, ids)
}

// AddPaths merges the paths into the current list of role paths
func (r *Role) AddPaths(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	r.Paths = mergeStrSlices(r.Paths, paths)
	return nil
}

// RemoveKeys removes the ids from the current list of key ids
func (r *Role) RemoveKeys(ids []string) {
	r.KeyIDs = subtractStrSlices(r.KeyIDs, ids)
}

// RemovePaths removes the paths from the current list of role paths
func (r *Role) RemovePaths(paths []string) {
	r.Paths = subtractStrSlices(r.Paths, paths)
}

func mergeStrSlices(orig, new []string) []string {
	have := make(map[string]bool)
	for _, e := range orig {
		have[e] = true
	}
	merged := make([]string, len(orig), len(orig)+len(new))
	copy(merged, orig)
	for _, e := range new {
		if !have[e] {
			merged = append(merged, e)
		}
	}
	return merged
}

func subtractStrSlices(orig, remove []string) []string {
	kill := make(map[string]bool)
	for _, e := range remove {
		kill[e] = true
	}
	var keep []string
	for _, e := range orig {
		if !kill[e] {
			keep = append(keep, e)
		}
	}
	return keep
}
