//go:build !windows

package usergroup

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/moby/sys/user"
)

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
	getentCmd, err := resolveBinary("getent")
	// if no `getent` command within the execution environment, can't do anything else
	if err != nil {
		return nil, fmt.Errorf("unable to find getent command: %w", err)
	}
	command := exec.Command(getentCmd, database, key)
	// we run getent within container filesystem, but without /dev so /dev/null is not available for exec to mock stdin
	command.Stdin = io.NopCloser(bytes.NewReader(nil))
	out, err := command.CombinedOutput()
	if err != nil {
		exitCode, errC := getExitCode(err)
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

// getExitCode returns the ExitStatus of the specified error if its type is
// exec.ExitError, returns 0 and an error otherwise.
func getExitCode(err error) (int, error) {
	exitCode := 0
	if exiterr, ok := err.(*exec.ExitError); ok {
		if procExit, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return procExit.ExitStatus(), nil
		}
	}
	return exitCode, fmt.Errorf("failed to get exit code")
}

// LoadIdentityMapping takes a requested username and
// using the data from /etc/sub{uid,gid} ranges, creates the
// proper uid and gid remapping ranges for that user/group pair
func LoadIdentityMapping(name string) (user.IdentityMapping, error) {
	usr, err := LookupUser(name)
	if err != nil {
		return user.IdentityMapping{}, fmt.Errorf("could not get user for username %s: %v", name, err)
	}

	subuidRanges, err := lookupSubRangesFile("/etc/subuid", usr)
	if err != nil {
		return user.IdentityMapping{}, err
	}
	subgidRanges, err := lookupSubRangesFile("/etc/subgid", usr)
	if err != nil {
		return user.IdentityMapping{}, err
	}

	return user.IdentityMapping{
		UIDMaps: subuidRanges,
		GIDMaps: subgidRanges,
	}, nil
}

func lookupSubRangesFile(path string, usr user.User) ([]user.IDMap, error) {
	uidstr := strconv.Itoa(usr.Uid)
	rangeList, err := user.ParseSubIDFileFilter(path, func(sid user.SubID) bool {
		return sid.Name == usr.Name || sid.Name == uidstr
	})
	if err != nil {
		return nil, err
	}
	if len(rangeList) == 0 {
		return nil, fmt.Errorf("no subuid ranges found for user %q", usr.Name)
	}

	idMap := []user.IDMap{}

	containerID := int64(0)
	for _, idrange := range rangeList {
		idMap = append(idMap, user.IDMap{
			ID:       containerID,
			ParentID: idrange.SubID,
			Count:    idrange.Count,
		})
		containerID = containerID + idrange.Count
	}
	return idMap, nil
}
