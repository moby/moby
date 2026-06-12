/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package mount

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func parseIDMapping(mapping string) (syscall.SysProcIDMap, error) {
	var retval syscall.SysProcIDMap

	parts := strings.Split(mapping, ":")
	if len(parts) != 3 {
		return retval, fmt.Errorf("user namespace mappings require the format `container-id:host-id:size`")
	}

	cID, err := strconv.Atoi(parts[0])
	if err != nil {
		return retval, fmt.Errorf("invalid container id for user namespace remapping, %w", err)
	}

	hID, err := strconv.Atoi(parts[1])
	if err != nil {
		return retval, fmt.Errorf("invalid host id for user namespace remapping, %w", err)
	}

	size, err := strconv.Atoi(parts[2])
	if err != nil {
		return retval, fmt.Errorf("invalid size for user namespace remapping, %w", err)
	}

	if cID < 0 || hID < 0 || size < 0 {
		return retval, fmt.Errorf("invalid mapping %s, all IDs and size must be positive integers", mapping)
	}

	retval = syscall.SysProcIDMap{
		ContainerID: cID,
		HostID:      hID,
		Size:        size,
	}

	return retval, nil
}

func parseIDMappingList(mappings string) ([]syscall.SysProcIDMap, error) {
	var (
		res     []syscall.SysProcIDMap
		maplist = strings.Split(mappings, ",")
	)
	for _, m := range maplist {
		r, err := parseIDMapping(m)
		if err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, nil
}

// IDMapMount clones the mount at source to target, applying GID/UID idmapping of the user namespace for target path
func IDMapMount(source, target string, usernsFd int) (err error) {
	return IDMapMountWithAttrs(source, target, usernsFd, 0, 0)
}

// IDMapMountWithAttrs clones the mount at source to target with the provided mount options and idmapping of the user namespace.
func IDMapMountWithAttrs(source, target string, usernsFd int, attrSet uint64, attrClr uint64) (err error) {
	var (
		attr unix.MountAttr
	)

	attr.Attr_set = unix.MOUNT_ATTR_IDMAP | attrSet
	attr.Attr_clr = attrClr
	attr.Propagation = unix.MS_PRIVATE
	attr.Userns_fd = uint64(usernsFd)

	dFd, err := unix.OpenTree(-int(unix.EBADF), source, uint(unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC|unix.AT_EMPTY_PATH|unix.AT_RECURSIVE))
	if err != nil {
		return fmt.Errorf("unable to open tree for %s: %w", target, err)
	}

	defer unix.Close(dFd)
	if err = unix.MountSetattr(dFd, "", unix.AT_EMPTY_PATH|unix.AT_RECURSIVE, &attr); err != nil {
		return fmt.Errorf("unable to shift GID/UID or set mount attrs for %s: %w", target, err)
	}

	if err = unix.MoveMount(dFd, "", -int(unix.EBADF), target, unix.MOVE_MOUNT_F_EMPTY_PATH); err != nil {
		return fmt.Errorf("unable to attach mount tree to %s: %w", target, err)
	}
	return nil
}

// GetUsernsFD forks the current process and creates a user namespace using the specified mappings.
// Expected syntax of ID mapping parameter is "%d:%d:%d[,%d:%d:%d,...]"
func GetUsernsFD(uidmap, gidmap string) (_usernsFD *os.File, _ error) {
	uidMaps, err := parseIDMappingList(uidmap)
	if err != nil {
		return nil, err
	}

	gidMaps, err := parseIDMappingList(gidmap)
	if err != nil {
		return nil, err
	}
	return getUsernsFD(uidMaps, gidMaps)
}
