// +build !windows

package idtools // import "github.com/docker/docker/pkg/idtools"

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/docker/docker/pkg/system"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/pkg/errors"
)

var (
	entOnce   sync.Once
	getentCmd string
)

func mkdirAs(path string, mode os.FileMode, owner Identity, mkAll, chownExisting bool) error {
	// make an array containing the original path asked for, plus (for mkAll == true)
	// all path components leading up to the complete path that don't exist before we MkdirAll
	// so that we can chown all of them properly at the end.  If chownExisting is false, we won't
	// chown the full directory path if it exists

	var paths []string

	stat, err := system.Stat(path)
	if err == nil {
		if !stat.IsDir() {
			return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
		}
		if !chownExisting {
			return nil
		}

		// short-circuit--we were called with an existing directory and chown was requested
		return lazyChown(path, owner.UID, owner.GID, stat)
	}

	if os.IsNotExist(err) {
		paths = []string{path}
	}

	if mkAll {
		// walk back to "/" looking for directories which do not exist
		// and add them to the paths array for chown after creation
		dirPath := path
		for {
			dirPath = filepath.Dir(dirPath)
			if dirPath == "/" {
				break
			}
			if _, err := os.Stat(dirPath); err != nil && os.IsNotExist(err) {
				paths = append(paths, dirPath)
			}
		}
		if err := system.MkdirAll(path, mode); err != nil {
			return err
		}
	} else {
		if err := os.Mkdir(path, mode); err != nil && !os.IsExist(err) {
			return err
		}
	}
	// even if it existed, we will chown the requested path + any subpaths that
	// didn't exist when we called MkdirAll
	for _, pathComponent := range paths {
		if err := lazyChown(pathComponent, owner.UID, owner.GID, nil); err != nil {
			return err
		}
	}
	return nil
}

// CanAccess takes a valid (existing) directory and a uid, gid pair and determines
// if that uid, gid pair has access (execute bit) to the directory
func CanAccess(path string, pair Identity) bool {
	statInfo, err := system.Stat(path)
	if err != nil {
		return false
	}
	fileMode := os.FileMode(statInfo.Mode())
	permBits := fileMode.Perm()
	return accessible(statInfo.UID() == uint32(pair.UID),
		statInfo.GID() == uint32(pair.GID), permBits)
}

func accessible(isOwner, isGroup bool, perms os.FileMode) bool {
	if isOwner && (perms&0100 == 0100) {
		return true
	}
	if isGroup && (perms&0010 == 0010) {
		return true
	}
	if perms&0001 == 0001 {
		return true
	}
	return false
}

// LookupUser uses traditional local system files lookup (from libcontainer/user) on a username,
// followed by a call to `getent` for supporting host configured non-files passwd and group dbs
func LookupUser(name string) (user.User, error) {
	// first try a local system files lookup using existing capabilities
	usr, err := user.LookupUser(name)
	if err == nil {
		return usr, nil
	}
	// local files lookup failed; attempt to call `getent` to query configured passwd dbs
	usr, err = getentUser(name)
	if err != nil {
		return user.User{}, err
	}
	return usr, nil
}

// LookupUID uses traditional local system files lookup (from libcontainer/user) on a uid,
// followed by a call to `getent` for supporting host configured non-files passwd and group dbs
func LookupUID(uid int) (user.User, error) {
	// first try a local system files lookup using existing capabilities
	usr, err := user.LookupUid(uid)
	if err == nil {
		return usr, nil
	}
	// local files lookup failed; attempt to call `getent` to query configured passwd dbs
	return getentUser(strconv.Itoa(uid))
}

func getentUser(name string) (user.User, error) {
	reader, err := callGetent("passwd", name)
	if err != nil {
		return user.User{}, err
	}
	users, err := user.ParsePasswd(reader)
	if err != nil {
		return user.User{}, err
	}
	if len(users) == 0 {
		return user.User{}, fmt.Errorf("getent failed to find passwd entry for %q", name)
	}
	return users[0], nil
}

// LookupGroup uses traditional local system files lookup (from libcontainer/user) on a group name,
// followed by a call to `getent` for supporting host configured non-files passwd and group dbs
func LookupGroup(name string) (user.Group, error) {
	// first try a local system files lookup using existing capabilities
	group, err := user.LookupGroup(name)
	if err == nil {
		return group, nil
	}
	// local files lookup failed; attempt to call `getent` to query configured group dbs
	return getentGroup(name)
}

// LookupGID uses traditional local system files lookup (from libcontainer/user) on a group ID,
// followed by a call to `getent` for supporting host configured non-files passwd and group dbs
func LookupGID(gid int) (user.Group, error) {
	// first try a local system files lookup using existing capabilities
	group, err := user.LookupGid(gid)
	if err == nil {
		return group, nil
	}
	// local files lookup failed; attempt to call `getent` to query configured group dbs
	return getentGroup(strconv.Itoa(gid))
}

func getentGroup(name string) (user.Group, error) {
	reader, err := callGetent("group", name)
	if err != nil {
		return user.Group{}, err
	}
	groups, err := user.ParseGroup(reader)
	if err != nil {
		return user.Group{}, err
	}
	if len(groups) == 0 {
		return user.Group{}, fmt.Errorf("getent failed to find groups entry for %q", name)
	}
	return groups[0], nil
}

func callGetent(database, key string) (io.Reader, error) {
	entOnce.Do(func() { getentCmd, _ = resolveBinary("getent") })
	// if no `getent` command on host, can't do anything else
	if getentCmd == "" {
		return nil, fmt.Errorf("unable to find getent command")
	}
	out, err := execCmd(getentCmd, database, key)
	if err != nil {
		exitCode, errC := system.GetExitCode(err)
		if errC != nil {
			return nil, err
		}
		switch exitCode {
		case 1:
			return nil, fmt.Errorf("getent reported invalid parameters/database unknown")
		case 2:
			return nil, fmt.Errorf("getent unable to find entry %q in %s database", key, database)
		case 3:
			return nil, fmt.Errorf("getent database doesn't support enumeration")
		default:
			return nil, err
		}

	}
	return bytes.NewReader(out), nil
}

// lazyChown performs a chown only if the uid/gid don't match what's requested
// Normally a Chown is a no-op if uid/gid match, but in some cases this can still cause an error, e.g. if the
// dir is on an NFS share, so don't call chown unless we absolutely must.
func lazyChown(p string, uid, gid int, stat *system.StatT) error {
	if stat == nil {
		var err error
		stat, err = system.Stat(p)
		if err != nil {
			return err
		}
	}
	if stat.UID() == uint32(uid) && stat.GID() == uint32(gid) {
		return nil
	}
	return os.Chown(p, uid, gid)
}

// NewIdentityMapping takes a requested username and
// using the data from /etc/sub{uid,gid} ranges, creates the
// proper uid and gid remapping ranges for that user/group pair
func NewIdentityMapping(name string) (*IdentityMapping, error) {
	usr, err := LookupUser(name)
	if err != nil {
		return nil, fmt.Errorf("Could not get user for username %s: %v", name, err)
	}

	uid := strconv.Itoa(usr.Uid)

	subuidRangesWithUserName, err := parseSubuid(name)
	if err != nil {
		return nil, err
	}
	subgidRangesWithUserName, err := parseSubgid(name)
	if err != nil {
		return nil, err
	}

	subuidRangesWithUID, err := parseSubuid(uid)
	if err != nil {
		return nil, err
	}
	subgidRangesWithUID, err := parseSubgid(uid)
	if err != nil {
		return nil, err
	}

	subuidRanges := append(subuidRangesWithUserName, subuidRangesWithUID...)
	subgidRanges := append(subgidRangesWithUserName, subgidRangesWithUID...)

	if len(subuidRanges) == 0 {
		return nil, errors.Errorf("no subuid ranges found for user %q", name)
	}
	if len(subgidRanges) == 0 {
		return nil, errors.Errorf("no subgid ranges found for user %q", name)
	}

	return &IdentityMapping{
		uids: createIDMap(subuidRanges),
		gids: createIDMap(subgidRanges),
	}, nil
}
