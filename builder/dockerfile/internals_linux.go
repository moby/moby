package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/moby/sys/symlink"
	"github.com/moby/sys/user"
	"github.com/pkg/errors"
)

func parseChownFlag(ctx context.Context, builder *Builder, state *dispatchState, chown, ctrRootPath string, identityMapping user.IdentityMapping) (identity, error) {
	var userStr, grpStr string
	parts := strings.Split(chown, ":")
	if len(parts) > 2 {
		return identity{}, errors.New("invalid chown string format: " + chown)
	}
	if len(parts) == 1 {
		// if no group specified, use the user spec as group as well
		userStr, grpStr = parts[0], parts[0]
	} else {
		userStr, grpStr = parts[0], parts[1]
	}

	passwdPath, err := symlink.FollowSymlinkInScope(filepath.Join(ctrRootPath, "etc", "passwd"), ctrRootPath)
	if err != nil {
		return identity{}, errors.Wrap(err, "can't resolve /etc/passwd path in container rootfs")
	}
	groupPath, err := symlink.FollowSymlinkInScope(filepath.Join(ctrRootPath, "etc", "group"), ctrRootPath)
	if err != nil {
		return identity{}, errors.Wrap(err, "can't resolve /etc/group path in container rootfs")
	}
	uid, err := lookupUser(userStr, passwdPath)
	if err != nil {
		return identity{}, errors.Wrap(err, "can't find uid for user "+userStr)
	}
	gid, err := lookupGroup(grpStr, groupPath)
	if err != nil {
		return identity{}, errors.Wrap(err, "can't find gid for group "+grpStr)
	}

	// convert as necessary because of user namespaces
	uid, gid, err = identityMapping.ToHost(uid, gid)
	if err != nil {
		return identity{}, errors.Wrap(err, "unable to convert uid/gid to host mapping")
	}
	return identity{UID: uid, GID: gid}, nil
}

func lookupUser(userStr, filepath string) (int, error) {
	// if the string is actually a uid integer, parse to int and return
	// as we don't need to translate with the help of files
	uid, err := strconv.Atoi(userStr)
	if err == nil {
		return uid, nil
	}
	users, err := user.ParsePasswdFileFilter(filepath, func(u user.User) bool {
		return u.Name == userStr
	})
	if err != nil {
		return 0, err
	}
	if len(users) == 0 {
		return 0, errors.New("no such user: " + userStr)
	}
	return users[0].Uid, nil
}

func lookupGroup(groupStr, filepath string) (int, error) {
	// if the string is actually a gid integer, parse to int and return
	// as we don't need to translate with the help of files
	gid, err := strconv.Atoi(groupStr)
	if err == nil {
		return gid, nil
	}
	groups, err := user.ParseGroupFileFilter(filepath, func(g user.Group) bool {
		return g.Name == groupStr
	})
	if err != nil {
		return 0, err
	}
	if len(groups) == 0 {
		return 0, errors.New("no such group: " + groupStr)
	}
	return groups[0].Gid, nil
}
