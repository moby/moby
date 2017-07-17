//go:build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"math"
	"strconv"
	"strings"

	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/sys/user"
	"github.com/pkg/errors"
)

func (daemon *Daemon) tarCopyOptions(ctr *container.Container, noOverwriteDirNonDir bool) (*archive.TarOptions, error) {
	if ctr.Config.User == "" {
		return daemon.defaultTarCopyOptions(noOverwriteDirNonDir), nil
	}

	uid, gid, err := getUIDGID(ctr)
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	return &archive.TarOptions{
		NoOverwriteDirNonDir: noOverwriteDirNonDir,
		ChownOpts:            &idtools.Identity{UID: uid, GID: gid},
	}, nil
}

// getUIDGID resolves the UID and GID of a given container's Config.User,
// which can contain a user name (or ID) and, optionally, group (or ID).
//
// usergrp is a username or uid, and optional group, in the format `user[:group]`.
// Both `user` and `group` can be provided as an `uid` / `gid`, so the following
// formats are supported:
//
// - username            - valid username from getent(1)
// - username:groupname  - valid username; valid groupname from /etc/passwd
// - uid                 - 32-bit unsigned int valid Linux UID value
// - uid:gid             - uid value; 32-bit unsigned int Linux GID value
// - username:gid        - valid username from getent(1), gid value; 32-bit unsigned int Linux GID value
// - uid:groupname       - 32-bit unsigned int valid Linux UID value, valid groupname from /etc/group
//
// If only a username (or uid) is provided, an attempt is made to lookup the gid
// for that username using /etc/passwd
func getUIDGID(ctr *container.Container) (uid int, gid int, _ error) {
	userNameOrID, groupNameOrID, hasGroup := strings.Cut(ctr.Config.User, ":")

	var (
		userID, userName   = getIDOrName(userNameOrID)
		groupID, groupName = getIDOrName(groupNameOrID)
	)
	if userName == "" && hasGroup && groupName == "" {
		// UID/GID passed; nothing to look up.
		return userID, groupID, nil
	}

	usr, err := lookupUser(ctr, userNameOrID)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "failed to look up user %q in container: %w", userNameOrID)
	}
	if !hasGroup || groupNameOrID == "" {
		// Use user's primary group
		return usr.Uid, usr.Gid, nil
	}

	groupID, err = lookupGID(ctr, groupNameOrID)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "failed to look up group %q in container: %w", groupNameOrID)
	}
	return usr.Uid, groupID, nil
}

// getIDOrName checks whether nameOrID is a ID (integer) or a Name.
// It assumes nameOrID is a name when failing to parse as integer,
// in which case a non-empty name is returned.
func getIDOrName(nameOrID string) (id int, name string) {
	if uid, err := strconv.ParseUint(nameOrID, 10, 32); err == nil && uid <= math.MaxInt32 {
		// uid provided
		return int(uid), ""
	}
	// not am id, assume name
	return 0, nameOrID
}

func lookupUser(ctr *container.Container, nameOrID string) (user.User, error) {
	passwdPath, err := resourcePath(ctr, user.GetPasswdPath)
	if err != nil {
		return user.User{}, err
	}
	userID, userName := getIDOrName(nameOrID)
	users, err := user.ParsePasswdFileFilter(passwdPath, func(entry user.User) bool {
		if userName != "" {
			return entry.Name == userName
		}
		return entry.Uid == userID
	})
	if err != nil {
		return user.User{}, err
	}
	if len(users) == 0 {
		return user.User{}, errors.New("no group found")
	}
	return users[0], nil
}

func lookupGID(ctr *container.Container, nameOrID string) (int, error) {
	groupID, groupName := getIDOrName(nameOrID)
	if groupName == "" {
		// GID is passed, no need to look up
		return groupID, nil
	}

	groupPath, err := resourcePath(ctr, user.GetGroupPath)
	if err != nil {
		return 0, err
	}
	groups, err := user.ParseGroupFileFilter(groupPath, func(entry user.Group) bool {
		if groupName != "" {
			return entry.Name == groupName
		}
		return entry.Gid == groupID
	})
	if err != nil {
		return 0, err
	}
	if len(groups) == 0 {
		return 0, errors.New("no group found")
	}
	return groups[0].Gid, nil
}
