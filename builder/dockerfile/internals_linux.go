package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/symlink"
	lcUser "github.com/opencontainers/runc/libcontainer/user"
	"github.com/pkg/errors"
)

func parseChownFlag(chown, ctrRootPath string, idMappings *idtools.IDMappings) (idtools.IDPair, error) {
	var userStr, grpStr string
	parts := strings.Split(chown, ":")
	if len(parts) > 2 {
		return idtools.IDPair{}, errors.New("invalid chown string format: " + chown)
	}
	if len(parts) == 1 {
		// if no group specified, use the user spec as group as well
		userStr, grpStr = parts[0], parts[0]
	} else {
		userStr, grpStr = parts[0], parts[1]
	}

	passwdPath, err := symlink.FollowSymlinkInScope(filepath.Join(ctrRootPath, "etc", "passwd"), ctrRootPath)
	if err != nil {
		return idtools.IDPair{}, errors.Wrapf(err, "can't resolve /etc/passwd path in container rootfs")
	}
	groupPath, err := symlink.FollowSymlinkInScope(filepath.Join(ctrRootPath, "etc", "group"), ctrRootPath)
	if err != nil {
		return idtools.IDPair{}, errors.Wrapf(err, "can't resolve /etc/group path in container rootfs")
	}
	uid, err := lookupUser(userStr, passwdPath)
	if err != nil {
		return idtools.IDPair{}, errors.Wrapf(err, "can't find uid for user "+userStr)
	}
	gid, err := lookupGroup(grpStr, groupPath)
	if err != nil {
		return idtools.IDPair{}, errors.Wrapf(err, "can't find gid for group "+grpStr)
	}

	// convert as necessary because of user namespaces
	chownPair, err := idMappings.ToHost(idtools.IDPair{UID: uid, GID: gid})
	if err != nil {
		return idtools.IDPair{}, errors.Wrapf(err, "unable to convert uid/gid to host mapping")
	}
	return chownPair, nil
}

func parseChmodFlag(chmodStr string) (uint16, error) {
	parsedVal, err := strconv.ParseUint(chmodStr, 8, 64)
	if err != nil {
		return chmodSymbolsToInt(chmodStr)
	}

	return uint16(parsedVal), nil
}

func chmodSymbolsToInt(chmodStr string) (uint16, error) {
	perms := map[string]uint{
		"u": 0,
		"g": 0,
		"o": 0,
	}

	chmodBits := map[string]uint{
		"r": 4, //100
		"w": 2, //010
		"x": 1, //001
	}

	tokenizers := []string{"=", "+", "-"}
	for _, segment := range strings.Split(chmodStr, ",") {
		for _, delimiter := range tokenizers {
			values := strings.Split(segment, delimiter)
			if len(values) == 1 {
				continue
			}

			for _, subject := range strings.Split(values[0], "") {
				for _, symbol := range strings.Split(values[1], "") {
					val, ok := chmodBits[symbol]
					if !ok {
						return 0, errors.New("invalid chmod flags " + chmodStr)
					}
					switch delimiter {
					case "=":
						if subject == "a" {
							perms["u"] = val
							perms["g"] = val
							perms["0"] = val
						} else {
							perms[subject] = val
						}
					case "+":
						if subject == "a" {
							perms["u"] |= val
							perms["g"] |= val
							perms["o"] |= val
						} else {
							perms[subject] |= val
						}
					case "-":
						if subject == "a" {
							perms["u"] &= ^val
							perms["g"] &= ^val
							perms["o"] &= ^val
						} else {
							perms[subject] &= ^val
						}
					default:
						return 0, errors.New("invalid chmod flag " + chmodStr)
					}
				}
			}
		}
	}

	val, _ := strconv.ParseUint(fmt.Sprintf("%d%d%d", perms["u"], perms["g"], perms["o"]), 8, 64)
	return uint16(val), nil
}

func lookupUser(userStr, filepath string) (int, error) {
	// if the string is actually a uid integer, parse to int and return
	// as we don't need to translate with the help of files
	uid, err := strconv.Atoi(userStr)
	if err == nil {
		return uid, nil
	}
	users, err := lcUser.ParsePasswdFileFilter(filepath, func(u lcUser.User) bool {
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
	groups, err := lcUser.ParseGroupFileFilter(filepath, func(g lcUser.Group) bool {
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
