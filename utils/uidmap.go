package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dotcloud/docker/pkg/user"
)

func xlateOneFile(path string, finfo os.FileInfo, containerRoot uint32, inverse bool) error {
	uid := uint32(finfo.Sys().(*syscall.Stat_t).Uid)
	gid := uint32(finfo.Sys().(*syscall.Stat_t).Gid)
	mode := finfo.Mode()

	if ((uid == 0 || gid == 0) && !inverse) || ((uid == containerRoot || gid == containerRoot) && inverse) {
		newUid := uid
		newGid := gid
		if uid == 0 && !inverse {
			newUid = containerRoot
		}
		if gid == 0 && !inverse {
			newGid = containerRoot
		}
		if uid == containerRoot && inverse {
			newUid = 0
		}
		if gid == containerRoot && inverse {
			newGid = 0
		}
		if err := os.Lchown(path, int(newUid), int(newGid)); err != nil {
			return fmt.Errorf("Cannot chown %s: %s", path, err)
		}
		if err := os.Chmod(path, mode); err != nil {
			return fmt.Errorf("Cannot chmod %s: %s", path, err)
		}
	}

	return nil
}

func xlateUidsRecursive(base string, containerRoot uint32, inverse bool) error {
	f, err := os.Open(base)
	if err != nil {
		return err
	}

	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return err
	}

	for _, finfo := range list {
		path := filepath.Join(base, finfo.Name())
		if finfo.IsDir() {
			xlateUidsRecursive(path, containerRoot, inverse)
		}
		if err := xlateOneFile(path, finfo, containerRoot, inverse); err != nil {
			return err
		}
	}

	return nil
}

// Chown any root files to docker-root
func XlateUids(root string, inverse bool) error {
	containerRoot, err := ContainerRootUid()
	if err != nil {
		return err
	}
	if err := xlateUidsRecursive(root, containerRoot, inverse); err != nil {
		return err
	}
	finfo, err := os.Stat(root)
	if err != nil {
		return err
	}
	if err := xlateOneFile(root, finfo, containerRoot, inverse); err != nil {
		return err
	}

	return nil
}

// Get the uid of docker-root user
func ContainerRootUid() (uint32, error) {
	uid, _, _, err := user.GetUserGroupSupplementary("docker-root", syscall.Getuid(), syscall.Getgid())
	if err != nil {
		return 0, err
	}

	return uint32(uid), nil
}

// Get the highest uid on the host from /proc
func HostMaxUid() (uint32, error) {
	file, err := os.Open("/proc/self/uid_map")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	uidMapString := make([]byte, 100)
	_, err = file.Read(uidMapString)
	if err != nil {
		return 0, err
	}

	var tmp, maxUid uint32
	fmt.Sscanf(string(uidMapString), "%d %d %d", &tmp, &tmp, &maxUid)

	return maxUid, nil
}
