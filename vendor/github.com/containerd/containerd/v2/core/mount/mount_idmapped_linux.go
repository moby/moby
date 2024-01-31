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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/containerd/containerd/v2/pkg/sys"
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

func getUsernsFD(uidMaps, gidMaps []syscall.SysProcIDMap) (_usernsFD *os.File, retErr error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pid, pidfd, errno := sys.ForkUserns()
	if errno != 0 {
		return nil, errno
	}

	pidFD := os.NewFile(pidfd, "pidfd")
	defer func() {
		unix.PidfdSendSignal(int(pidFD.Fd()), unix.SIGKILL, nil, 0)

		pidfdWaitid(pidFD)

		pidFD.Close()
	}()

	// NOTE:
	//
	// The usernsFD will hold the userns reference in kernel. Even if the
	// child process is reaped, the usernsFD is still valid.
	usernsFD, err := os.Open(fmt.Sprintf("/proc/%d/ns/user", pid))
	if err != nil {
		return nil, fmt.Errorf("failed to get userns file descriptor for /proc/%d/user/ns: %w", pid, err)
	}
	defer func() {
		if retErr != nil {
			usernsFD.Close()
		}
	}()

	uidmapFile, err := os.OpenFile(fmt.Sprintf("/proc/%d/%s", pid, "uid_map"), os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open /proc/%d/uid_map: %w", pid, err)
	}
	defer uidmapFile.Close()

	gidmapFile, err := os.OpenFile(fmt.Sprintf("/proc/%d/%s", pid, "gid_map"), os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open /proc/%d/gid_map: %w", pid, err)
	}
	defer gidmapFile.Close()

	testHookKillChildBeforePidfdSendSignal(pid, pidFD)

	// Ensure the child process is still alive. If the err is ESRCH, we
	// should return error because we can't guarantee the usernsFD and
	// u[g]idmapFile are valid. It's safe to return error and retry.
	if err := unix.PidfdSendSignal(int(pidFD.Fd()), 0, nil, 0); err != nil {
		return nil, fmt.Errorf("failed to ensure child process is alive: %w", err)
	}

	testHookKillChildAfterPidfdSendSignal(pid, pidFD)

	// NOTE:
	//
	// The u[g]id_map file descriptor is still valid if the child process
	// is reaped.
	writeMappings := func(f *os.File, idmap []syscall.SysProcIDMap) error {
		mappings := ""
		for _, m := range idmap {
			mappings = fmt.Sprintf("%s%d %d %d\n", mappings, m.ContainerID, m.HostID, m.Size)
		}

		_, err := f.Write([]byte(mappings))
		if err1 := f.Close(); err1 != nil && err == nil {
			err = err1
		}
		return err
	}

	if err := writeMappings(uidmapFile, uidMaps); err != nil {
		return nil, fmt.Errorf("failed to write uid_map: %w", err)
	}

	if err := writeMappings(gidmapFile, gidMaps); err != nil {
		return nil, fmt.Errorf("failed to write gid_map: %w", err)
	}
	return usernsFD, nil
}

func pidfdWaitid(pidFD *os.File) error {
	// https://elixir.bootlin.com/linux/v5.4.258/source/include/uapi/linux/wait.h#L20
	const PPidFD = 3

	var e syscall.Errno
	for {
		_, _, e = syscall.Syscall6(syscall.SYS_WAITID, PPidFD, pidFD.Fd(), 0, syscall.WEXITED, 0, 0)
		if e != syscall.EINTR {
			break
		}
	}
	return e
}

var (
	testHookLock sync.Mutex

	testHookKillChildBeforePidfdSendSignal = func(_pid uintptr, _pidFD *os.File) {}

	testHookKillChildAfterPidfdSendSignal = func(_pid uintptr, _pidFD *os.File) {}
)
