//go:build !windows

package daemon

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/moby/go-archive"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/errdefs"
	"github.com/moby/sys/user"
)

func (daemon *Daemon) tarCopyOptions(ctr *container.Container, allowOverwriteDirWithFile bool) (*archive.TarOptions, error) {
	if ctr.Config.User == "" {
		return daemon.defaultTarCopyOptions(allowOverwriteDirWithFile), nil
	}

	uid, gid, err := getUIDGID(ctr.Config.User)
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	return &archive.TarOptions{
		NoOverwriteDirNonDir: !allowOverwriteDirWithFile,
		ChownOpts:            &archive.ChownOpts{UID: uid, GID: gid},
	}, nil
}

// getUIDGID resolves the UID and GID of a given container's Config.User,
// which can contain a user name (or ID) and, optionally, group (or ID).
//
// usergrp is a username or uid, and optional group, in the format `user[:group]`.
// Both `user` and `group` can be provided as an `uid` / `gid`, so the following
// formats are supported:
//
// - username            - valid username from /etc/passwd
// - username:groupname  - valid username; valid groupname from /etc/passwd, /etc/group
// - uid                 - 32-bit unsigned int valid Linux UID value
// - uid:gid             - uid value; 32-bit unsigned int Linux GID value
// - username:gid        - valid username from getent(1), gid value; 32-bit unsigned int Linux GID value
// - uid:groupname       - 32-bit unsigned int valid Linux UID value, valid groupname from /etc/group
//
// If only a username (or uid) is provided, an attempt is made to look up the gid
// for that username using /etc/passwd
func getUIDGID(ctrUser string) (uid int, gid int, _ error) {
	userNameOrID, groupNameOrID, _ := strings.Cut(ctrUser, ":")

	// Align with behavior of docker run, which treats an empty username
	// or groupname as default (0 (root)).
	//
	//	docker run --rm --user ":33" alpine id
	//	uid=0(root) gid=33 groups=33
	//
	//	docker run --rm --user "33:" alpine id
	//	uid=33 gid=0(root) groups=0(root)
	if userNameOrID != "" {
		var err error
		uid, gid, err = lookupUser(userNameOrID)
		if err != nil {
			return 0, 0, err
		}
	}
	if groupNameOrID != "" {
		var err error
		gid, err = lookupGID(groupNameOrID)
		if err != nil {
			return 0, 0, err
		}
	}
	return uid, gid, nil
}

// getIDOrName checks whether nameOrID is a ID (integer) or a Name.
// It assumes nameOrID is a name when failing to parse as integer,
// in which case a non-empty name is returned.
func getIDOrName(nameOrID string) (id int, name string) {
	if uid, err := strconv.ParseUint(nameOrID, 10, 32); err == nil && uid <= math.MaxInt32 {
		// uid provided
		return int(uid), ""
	}
	// not an id, assume name
	return 0, nameOrID
}

func lookupUser(nameOrID string) (uid, gid int, _ error) {
	userID, userName := getIDOrName(nameOrID)
	if userName != "" {
		u, err := user.LookupUser(userName)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to look up user %q in container: %w", userName, err)
		}
		return u.Uid, u.Gid, nil
	}

	u, err := user.LookupUid(userID)
	if err != nil {
		// Match behavior of "docker run": when using a UID for the
		// user, resolving the user and its primary group is best-effort.
		// If a user with the given UID is found, we use its primary
		// group, otherwise use it as-is and use the default (0) as
		//  GID.
		//
		//	docker run --rm --user 12345 ubuntu id
		//	uid=12345 gid=0(root) groups=0(root)
		//
		//	docker run --rm ubuntu cat /etc/passwd | grep www-data
		//	www-data:x:33:33:www-data:/var/www:/usr/sbin/nologin
		//
		//	docker run --rm --user 33 ubuntu id
		//	uid=33(www-data) gid=33(www-data) groups=33(www-data)
		return userID, 0, nil
	}
	return u.Uid, u.Gid, nil
}

func lookupGID(nameOrID string) (int, error) {
	groupID, groupName := getIDOrName(nameOrID)
	if groupName == "" {
		// GID is passed, no need to look up
		return groupID, nil
	}
	group, err := user.LookupGroup(groupName)
	if err != nil {
		return 0, fmt.Errorf("failed to look up group %q in container: %w", nameOrID, err)
	}
	return group.Gid, nil
}
