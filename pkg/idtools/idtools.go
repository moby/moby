package idtools

import (
	"fmt"
	"os"

	"github.com/moby/sys/user"
)

// IDMap contains a single entry for user namespace range remapping. An array
// of IDMap entries represents the structure that will be provided to the Linux
// kernel for creating a user namespace.
//
// Deprecated: use [user.IDMap] instead.
type IDMap struct {
	ContainerID int `json:"container_id"`
	HostID      int `json:"host_id"`
	Size        int `json:"size"`
}

// MkdirAllAndChown creates a directory (include any along the path) and then modifies
// ownership to the requested uid/gid.  If the directory already exists, this
// function will still change ownership and permissions.
//
// Deprecated: use [user.MkdirAllAndChown] instead.
func MkdirAllAndChown(path string, mode os.FileMode, owner Identity) error {
	return user.MkdirAllAndChown(path, mode, owner.UID, owner.GID)
}

// MkdirAndChown creates a directory and then modifies ownership to the requested uid/gid.
// If the directory already exists, this function still changes ownership and permissions.
// Note that unlike os.Mkdir(), this function does not return IsExist error
// in case path already exists.
//
// Deprecated: use [user.MkdirAndChown] instead.
func MkdirAndChown(path string, mode os.FileMode, owner Identity) error {
	return user.MkdirAndChown(path, mode, owner.UID, owner.GID)
}

// MkdirAllAndChownNew creates a directory (include any along the path) and then modifies
// ownership ONLY of newly created directories to the requested uid/gid. If the
// directories along the path exist, no change of ownership or permissions will be performed
//
// Deprecated: use [user.MkdirAllAndChown] with the [user.WithOnlyNew] option instead.
func MkdirAllAndChownNew(path string, mode os.FileMode, owner Identity) error {
	return user.MkdirAllAndChown(path, mode, owner.UID, owner.GID, user.WithOnlyNew)
}

// GetRootUIDGID retrieves the remapped root uid/gid pair from the set of maps.
// If the maps are empty, then the root uid/gid will default to "real" 0/0
//
// Deprecated: use [(user.IdentityMapping).RootPair] instead.
func GetRootUIDGID(uidMap, gidMap []IDMap) (int, int, error) {
	return getRootUIDGID(uidMap, gidMap)
}

// getRootUIDGID retrieves the remapped root uid/gid pair from the set of maps.
// If the maps are empty, then the root uid/gid will default to "real" 0/0
func getRootUIDGID(uidMap, gidMap []IDMap) (int, int, error) {
	uid, err := toHost(0, uidMap)
	if err != nil {
		return -1, -1, err
	}
	gid, err := toHost(0, gidMap)
	if err != nil {
		return -1, -1, err
	}
	return uid, gid, nil
}

// toContainer takes an id mapping, and uses it to translate a
// host ID to the remapped ID. If no map is provided, then the translation
// assumes a 1-to-1 mapping and returns the passed in id
func toContainer(hostID int, idMap []IDMap) (int, error) {
	if idMap == nil {
		return hostID, nil
	}
	for _, m := range idMap {
		if (hostID >= m.HostID) && (hostID <= (m.HostID + m.Size - 1)) {
			contID := m.ContainerID + (hostID - m.HostID)
			return contID, nil
		}
	}
	return -1, fmt.Errorf("Host ID %d cannot be mapped to a container ID", hostID)
}

// toHost takes an id mapping and a remapped ID, and translates the
// ID to the mapped host ID. If no map is provided, then the translation
// assumes a 1-to-1 mapping and returns the passed in id #
func toHost(contID int, idMap []IDMap) (int, error) {
	if idMap == nil {
		return contID, nil
	}
	for _, m := range idMap {
		if (contID >= m.ContainerID) && (contID <= (m.ContainerID + m.Size - 1)) {
			hostID := m.HostID + (contID - m.ContainerID)
			return hostID, nil
		}
	}
	return -1, fmt.Errorf("Container ID %d cannot be mapped to a host ID", contID)
}

// Identity is either a UID and GID pair or a SID (but not both)
type Identity struct {
	UID int
	GID int
	SID string
}

// Chown changes the numeric uid and gid of the named file to id.UID and id.GID.
func (id Identity) Chown(name string) error {
	return os.Chown(name, id.UID, id.GID)
}

// IdentityMapping contains a mappings of UIDs and GIDs.
// The zero value represents an empty mapping.
type IdentityMapping struct {
	UIDMaps []IDMap `json:"UIDMaps"`
	GIDMaps []IDMap `json:"GIDMaps"`
}

// FromUserIdentityMapping converts a [user.IdentityMapping] to an [idtools.IdentityMapping].
//
// Deprecated: use [user.IdentityMapping] directly, this is transitioning to user package.
func FromUserIdentityMapping(u user.IdentityMapping) IdentityMapping {
	return IdentityMapping{
		UIDMaps: fromUserIDMap(u.UIDMaps),
		GIDMaps: fromUserIDMap(u.GIDMaps),
	}
}

func fromUserIDMap(u []user.IDMap) []IDMap {
	if u == nil {
		return nil
	}
	m := make([]IDMap, len(u))
	for i := range u {
		m[i] = IDMap{
			ContainerID: int(u[i].ID),
			HostID:      int(u[i].ParentID),
			Size:        int(u[i].Count),
		}
	}
	return m
}

// ToUserIdentityMapping converts an [idtools.IdentityMapping] to a [user.IdentityMapping].
//
// Deprecated: use [user.IdentityMapping] directly, this is transitioning to user package.
func ToUserIdentityMapping(u IdentityMapping) user.IdentityMapping {
	return user.IdentityMapping{
		UIDMaps: toUserIDMap(u.UIDMaps),
		GIDMaps: toUserIDMap(u.GIDMaps),
	}
}

func toUserIDMap(u []IDMap) []user.IDMap {
	if u == nil {
		return nil
	}
	m := make([]user.IDMap, len(u))
	for i := range u {
		m[i] = user.IDMap{
			ID:       int64(u[i].ContainerID),
			ParentID: int64(u[i].HostID),
			Count:    int64(u[i].Size),
		}
	}
	return m
}

// RootPair returns a uid and gid pair for the root user. The error is ignored
// because a root user always exists, and the defaults are correct when the uid
// and gid maps are empty.
func (i IdentityMapping) RootPair() Identity {
	uid, gid, _ := getRootUIDGID(i.UIDMaps, i.GIDMaps)
	return Identity{UID: uid, GID: gid}
}

// ToHost returns the host UID and GID for the container uid, gid.
// Remapping is only performed if the ids aren't already the remapped root ids
func (i IdentityMapping) ToHost(pair Identity) (Identity, error) {
	var err error
	target := i.RootPair()

	if pair.UID != target.UID {
		target.UID, err = toHost(pair.UID, i.UIDMaps)
		if err != nil {
			return target, err
		}
	}

	if pair.GID != target.GID {
		target.GID, err = toHost(pair.GID, i.GIDMaps)
	}
	return target, err
}

// ToContainer returns the container UID and GID for the host uid and gid
func (i IdentityMapping) ToContainer(pair Identity) (int, int, error) {
	uid, err := toContainer(pair.UID, i.UIDMaps)
	if err != nil {
		return -1, -1, err
	}
	gid, err := toContainer(pair.GID, i.GIDMaps)
	return uid, gid, err
}

// Empty returns true if there are no id mappings
func (i IdentityMapping) Empty() bool {
	return len(i.UIDMaps) == 0 && len(i.GIDMaps) == 0
}

// CurrentIdentity returns the identity of the current process
//
// Deprecated: use [os.Getuid] and [os.Getegid] instead.
func CurrentIdentity() Identity {
	return Identity{UID: os.Getuid(), GID: os.Getegid()}
}
