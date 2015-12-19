package data

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Canonical base role names
const (
	CanonicalRootRole      = "root"
	CanonicalTargetsRole   = "targets"
	CanonicalSnapshotRole  = "snapshot"
	CanonicalTimestampRole = "timestamp"
)

// ValidRoles holds an overrideable mapping of canonical role names
// to any custom roles names a user wants to make use of. This allows
// us to be internally consistent while using different roles in the
// public TUF files.
var ValidRoles = map[string]string{
	CanonicalRootRole:      CanonicalRootRole,
	CanonicalTargetsRole:   CanonicalTargetsRole,
	CanonicalSnapshotRole:  CanonicalSnapshotRole,
	CanonicalTimestampRole: CanonicalTimestampRole,
}

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

// SetValidRoles is a utility function to override some or all of the roles
func SetValidRoles(rs map[string]string) {
	// iterate ValidRoles
	for k := range ValidRoles {
		if v, ok := rs[k]; ok {
			ValidRoles[k] = v
		}
	}
}

// RoleName returns the (possibly overridden) role name for the provided
// canonical role name
func RoleName(canonicalRole string) string {
	if r, ok := ValidRoles[canonicalRole]; ok {
		return r
	}
	return canonicalRole
}

// CanonicalRole does a reverse lookup to get the canonical role name
// from the (possibly overridden) role name
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

	if IsDelegation(name) {
		return true
	}

	for _, v := range ValidRoles {
		if name == v {
			return true
		}
	}
	return false
}

// IsDelegation checks if the role is a delegation or a root role
func IsDelegation(role string) bool {
	targetsBase := ValidRoles[CanonicalTargetsRole] + "/"

	delegationRegexp := regexp.MustCompile("^[-a-z0-9_/]+$")
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

// RootRole is a cut down role as it appears in the root.json
type RootRole struct {
	KeyIDs    []string `json:"keyids"`
	Threshold int      `json:"threshold"`
}

// Role is a more verbose role as they appear in targets delegations
type Role struct {
	RootRole
	Name             string   `json:"name"`
	Paths            []string `json:"paths,omitempty"`
	PathHashPrefixes []string `json:"path_hash_prefixes,omitempty"`
	Email            string   `json:"email,omitempty"`
}

// NewRole creates a new Role object from the given parameters
func NewRole(name string, threshold int, keyIDs, paths, pathHashPrefixes []string) (*Role, error) {
	if len(paths) > 0 && len(pathHashPrefixes) > 0 {
		return nil, ErrInvalidRole{
			Role:   name,
			Reason: "roles may not have both Paths and PathHashPrefixes",
		}
	}
	if IsDelegation(name) {
		if len(paths) == 0 && len(pathHashPrefixes) == 0 {
			return nil, ErrInvalidRole{
				Role:   name,
				Reason: "roles with no Paths and no PathHashPrefixes will never be able to publish content",
			}
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
		Name:             name,
		Paths:            paths,
		PathHashPrefixes: pathHashPrefixes,
	}, nil

}

// IsValid checks if the role has defined both paths and path hash prefixes,
// having both is invalid
func (r Role) IsValid() bool {
	return !(len(r.Paths) > 0 && len(r.PathHashPrefixes) > 0)
}

// ValidKey checks if the given id is a recognized signing key for the role
func (r Role) ValidKey(id string) bool {
	for _, key := range r.KeyIDs {
		if key == id {
			return true
		}
	}
	return false
}

// CheckPaths checks if a given path is valid for the role
func (r Role) CheckPaths(path string) bool {
	for _, p := range r.Paths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// CheckPrefixes checks if a given hash matches the prefixes for the role
func (r Role) CheckPrefixes(hash string) bool {
	for _, p := range r.PathHashPrefixes {
		if strings.HasPrefix(hash, p) {
			return true
		}
	}
	return false
}

// IsDelegation checks if the role is a delegation or a root role
func (r Role) IsDelegation() bool {
	return IsDelegation(r.Name)
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
	if len(r.PathHashPrefixes) > 0 {
		return ErrInvalidRole{Role: r.Name, Reason: "attempted to add paths to role that already has hash prefixes"}
	}
	r.Paths = mergeStrSlices(r.Paths, paths)
	return nil
}

// AddPathHashPrefixes merges the prefixes into the list of role path hash prefixes
func (r *Role) AddPathHashPrefixes(prefixes []string) error {
	if len(prefixes) == 0 {
		return nil
	}
	if len(r.Paths) > 0 {
		return ErrInvalidRole{Role: r.Name, Reason: "attempted to add hash prefixes to role that already has paths"}
	}
	r.PathHashPrefixes = mergeStrSlices(r.PathHashPrefixes, prefixes)
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

// RemovePathHashPrefixes removes the prefixes from the current list of path hash prefixes
func (r *Role) RemovePathHashPrefixes(prefixes []string) {
	r.PathHashPrefixes = subtractStrSlices(r.PathHashPrefixes, prefixes)
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
