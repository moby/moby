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

// TODO: Support multiple mappings in future
func parseIDMapping(mapping string) ([]syscall.SysProcIDMap, error) {
	parts := strings.Split(mapping, ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("user namespace mappings require the format `container-id:host-id:size`")
	}

	cID, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid container id for user namespace remapping, %w", err)
	}

	hID, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid host id for user namespace remapping, %w", err)
	}

	size, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid size for user namespace remapping, %w", err)
	}

	if cID < 0 || hID < 0 || size < 0 {
		return nil, fmt.Errorf("invalid mapping %s, all IDs and size must be positive integers", mapping)
	}

	return []syscall.SysProcIDMap{
		{
			ContainerID: cID,
			HostID:      hID,
			Size:        size,
		},
	}, nil
}

// IDMapMount applies GID/UID shift according to gidmap/uidmap for target path
func IDMapMount(source, target string, usernsFd int) (err error) {
	var (
		attr unix.MountAttr
	)

	attr.Attr_set = unix.MOUNT_ATTR_IDMAP
	attr.Attr_clr = 0
	attr.Propagation = 0
	attr.Userns_fd = uint64(usernsFd)

	dFd, err := unix.OpenTree(-int(unix.EBADF), source, uint(unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC|unix.AT_EMPTY_PATH))
	if err != nil {
		return fmt.Errorf("Unable to open tree for %s: %w", target, err)
	}

	defer unix.Close(dFd)
	if err = unix.MountSetattr(dFd, "", unix.AT_EMPTY_PATH, &attr); err != nil {
		return fmt.Errorf("Unable to shift GID/UID for %s: %w", target, err)
	}

	if err = unix.MoveMount(dFd, "", -int(unix.EBADF), target, unix.MOVE_MOUNT_F_EMPTY_PATH); err != nil {
		return fmt.Errorf("Unable to attach mount tree to %s: %w", target, err)
	}
	return nil
}

// GetUsernsFD forks the current process and creates a user namespace using
// the specified mappings.
func GetUsernsFD(uidmap, gidmap string) (_usernsFD *os.File, _ error) {
	uidMaps, err := parseIDMapping(uidmap)
	if err != nil {
		return nil, err
	}

	gidMaps, err := parseIDMapping(gidmap)
	if err != nil {
		return nil, err
	}
	return getUsernsFD(uidMaps, gidMaps)
}
