package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"context"
	"strconv"
	"strings"

	"github.com/containerd/containerd/oci"
	"github.com/docker/docker/pkg/idtools"
	lcUser "github.com/opencontainers/runc/libcontainer/user"
	"github.com/pkg/errors"
)

func parseChownFlag(ctx context.Context, builder *Builder, state *dispatchState, chown, ctrRootPath string, identityMapping idtools.IdentityMapping) (idtools.Identity, error) {
	var userStr, grpStr string
	parts := strings.Split(chown, ":")
	if len(parts) > 2 {
		return idtools.Identity{}, errors.New("invalid chown string format: " + chown)
	}
	if len(parts) == 1 {
		// if no group specified, use the user spec as group as well
		userStr, grpStr = parts[0], parts[0]
	} else {
		userStr, grpStr = parts[0], parts[1]
	}

	uid, err := lookupUser(ctrRootPath, userStr)
	if err != nil {
		return idtools.Identity{}, errors.Wrapf(err, "can't find uid for user "+userStr)
	}
	gid, err := lookupGroup(ctrRootPath, grpStr)
	if err != nil {
		return idtools.Identity{}, errors.Wrapf(err, "can't find gid for group "+grpStr)
	}

	// convert as necessary because of user namespaces
	chownPair, err := identityMapping.ToHost(idtools.Identity{UID: uid, GID: gid})
	if err != nil {
		return idtools.Identity{}, errors.Wrapf(err, "unable to convert uid/gid to host mapping")
	}
	return chownPair, nil
}

func lookupUser(root, userStr string) (int, error) {
	// if the string is actually a uid integer, parse to int and return
	// as we don't need to translate with the help of files
	uid, err := strconv.Atoi(userStr)
	if err == nil {
		return uid, nil
	}
	user, err := oci.UserFromPath(root, func(u lcUser.User) bool {
		return u.Name == userStr
	})
	if err != nil {
		if errors.Is(err, oci.ErrNoUsersFound) {
			return 0, errors.New("no such user: " + userStr)
		}
		return 0, err
	}
	return user.Uid, nil
}

func lookupGroup(root, groupStr string) (int, error) {
	// if the string is actually a gid integer, parse to int and return
	// as we don't need to translate with the help of files
	gid, err := strconv.Atoi(groupStr)
	if err == nil {
		return gid, nil
	}
	group, err := oci.GIDFromPath(root, func(g lcUser.Group) bool {
		return g.Name == groupStr
	})
	if err != nil {
		if errors.Is(err, oci.ErrNoGroupsFound) {
			return 0, errors.New("no such group: " + groupStr)
		}
		return 0, err
	}
	return int(group), nil
}
