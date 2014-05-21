package daemon

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/daemon/execdriver"
	"github.com/dotcloud/docker/pkg/symlink"
)

type BindMap struct {
	SrcPath string
	DstPath string
	Mode    string
}

func prepareVolumesForContainer(container *Container) error {
	if container.Volumes == nil || len(container.Volumes) == 0 {
		container.Volumes = make(map[string]string)
		container.VolumesRW = make(map[string]bool)
		if err := applyVolumesFrom(container); err != nil {
			return err
		}
	}

	if err := createVolumes(container); err != nil {
		return err
	}
	return nil
}

func setupMountsForContainer(container *Container) error {
	mounts := []execdriver.Mount{
		{container.daemon.sysInitPath, "/.dockerinit", false, true},
		{container.ResolvConfPath, "/etc/resolv.conf", false, true},
	}

	if container.HostnamePath != "" {
		mounts = append(mounts, execdriver.Mount{container.HostnamePath, "/etc/hostname", false, true})
	}

	if container.HostsPath != "" {
		mounts = append(mounts, execdriver.Mount{container.HostsPath, "/etc/hosts", false, true})
	}

	// Mount user specified volumes
	// Note, these are not private because you may want propagation of (un)mounts from host
	// volumes. For instance if you use -v /usr:/usr and the host later mounts /usr/share you
	// want this new mount in the container
	for r, v := range container.Volumes {
		mounts = append(mounts, execdriver.Mount{v, r, container.VolumesRW[r], false})
	}

	container.command.Mounts = mounts

	return nil
}

func applyVolumesFrom(container *Container) error {
	volumesFrom := container.hostConfig.VolumesFrom
	if len(volumesFrom) > 0 {
		for _, containerSpec := range volumesFrom {
			var (
				mountRW   = true
				specParts = strings.SplitN(containerSpec, ":", 2)
			)

			switch len(specParts) {
			case 0:
				return fmt.Errorf("Malformed volumes-from specification: %s", containerSpec)
			case 2:
				switch specParts[1] {
				case "ro":
					mountRW = false
				case "rw": // mountRW is already true
				default:
					return fmt.Errorf("Malformed volumes-from specification: %s", containerSpec)
				}
			}

			c := container.daemon.Get(specParts[0])
			if c == nil {
				return fmt.Errorf("Container %s not found. Impossible to mount its volumes", specParts[0])
			}

			if err := c.Mount(); err != nil {
				return fmt.Errorf("Container %s failed to mount. Impossible to mount its volumes", specParts[0])
			}
			defer c.Unmount()

			for volPath, id := range c.Volumes {
				if _, exists := container.Volumes[volPath]; exists {
					continue
				}
				stat, err := os.Stat(c.getResourcePath(volPath))
				if err != nil {
					return err
				}
				if err := createIfNotExists(container.getResourcePath(volPath), stat.IsDir()); err != nil {
					return err
				}
				container.Volumes[volPath] = id
				if isRW, exists := c.VolumesRW[volPath]; exists {
					container.VolumesRW[volPath] = isRW && mountRW
				}
			}

		}
	}
	return nil
}

func getBindMap(container *Container) (map[string]BindMap, error) {
	var (
		// Create the requested bind mounts
		binds = make(map[string]BindMap)
		// Define illegal container destinations
		illegalDsts = []string{"/", "."}
	)

	for _, bind := range container.hostConfig.Binds {
		// FIXME: factorize bind parsing in parseBind
		var (
			src, dst, mode string
			arr            = strings.Split(bind, ":")
		)

		if len(arr) == 2 {
			src = arr[0]
			dst = arr[1]
			mode = "rw"
		} else if len(arr) == 3 {
			src = arr[0]
			dst = arr[1]
			mode = arr[2]
		} else {
			return nil, fmt.Errorf("Invalid bind specification: %s", bind)
		}

		// Bail if trying to mount to an illegal destination
		for _, illegal := range illegalDsts {
			if dst == illegal {
				return nil, fmt.Errorf("Illegal bind destination: %s", dst)
			}
		}

		bindMap := BindMap{
			SrcPath: src,
			DstPath: dst,
			Mode:    mode,
		}
		binds[filepath.Clean(dst)] = bindMap
	}
	return binds, nil
}

func createVolumes(container *Container) error {
	binds, err := getBindMap(container)
	if err != nil {
		return err
	}

	// Create the requested volumes if they don't exist
	for volPath := range container.Config.Volumes {
		if err := initializeVolume(container, volPath, binds); err != nil {
			return err
		}
	}

	for volPath := range binds {
		if err := initializeVolume(container, volPath, binds); err != nil {
			return err
		}
	}
	return nil
}

func createIfNotExists(path string, isDir bool) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if isDir {
				if err := os.MkdirAll(path, 0755); err != nil {
					return err
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					return err
				}
				f, err := os.OpenFile(path, os.O_CREATE, 0755)
				if err != nil {
					return err
				}
				defer f.Close()
			}
		}
	}
	return nil
}

func initializeVolume(container *Container, volPath string, binds map[string]BindMap) error {
	volumesDriver := container.daemon.volumes.Driver()
	volPath = filepath.Clean(volPath)
	// Skip existing volumes
	if _, exists := container.Volumes[volPath]; exists {
		return nil
	}

	var (
		srcPath     string
		isBindMount bool
		volIsDir    = true

		srcRW = false
	)

	// If an external bind is defined for this volume, use that as a source
	if bindMap, exists := binds[volPath]; exists {
		isBindMount = true
		srcPath = bindMap.SrcPath
		if !filepath.IsAbs(srcPath) {
			return fmt.Errorf("%s must be an absolute path", srcPath)
		}
		if strings.ToLower(bindMap.Mode) == "rw" {
			srcRW = true
		}
		if stat, err := os.Stat(bindMap.SrcPath); err != nil {
			return err
		} else {
			volIsDir = stat.IsDir()
		}
		// Otherwise create an directory in $ROOT/volumes/ and use that
	} else {
		// Do not pass a container as the parameter for the volume creation.
		// The graph driver using the container's information ( Image ) to
		// create the parent.
		c, err := container.daemon.volumes.Create(nil, "", "", "", "", nil, nil)
		if err != nil {
			return err
		}
		srcPath, err = volumesDriver.Get(c.ID, "")
		if err != nil {
			return fmt.Errorf("Driver %s failed to get volume rootfs %s: %s", volumesDriver, c.ID, err)
		}
		srcRW = true // RW by default
	}

	if p, err := filepath.EvalSymlinks(srcPath); err != nil {
		return err
	} else {
		srcPath = p
	}

	// Create the mountpoint
	rootVolPath, err := symlink.FollowSymlinkInScope(filepath.Join(container.basefs, volPath), container.basefs)
	if err != nil {
		return err
	}

	newVolPath, err := filepath.Rel(container.basefs, rootVolPath)
	if err != nil {
		return err
	}
	newVolPath = "/" + newVolPath

	if volPath != newVolPath {
		delete(container.Volumes, volPath)
		delete(container.VolumesRW, volPath)
	}

	container.Volumes[newVolPath] = srcPath
	container.VolumesRW[newVolPath] = srcRW

	if err := createIfNotExists(rootVolPath, volIsDir); err != nil {
		return err
	}

	// Do not copy or change permissions if we are mounting from the host
	if srcRW && !isBindMount {
		if err := copyExistingContents(rootVolPath, srcPath); err != nil {
			return err
		}
	}
	return nil
}

func copyExistingContents(rootVolPath, srcPath string) error {
	volList, err := ioutil.ReadDir(rootVolPath)
	if err != nil {
		return err
	}

	if len(volList) > 0 {
		srcList, err := ioutil.ReadDir(srcPath)
		if err != nil {
			return err
		}

		if len(srcList) == 0 {
			// If the source volume is empty copy files from the root into the volume
			if err := archive.CopyWithTar(rootVolPath, srcPath); err != nil {
				return err
			}
		}
	}

	var (
		stat    syscall.Stat_t
		srcStat syscall.Stat_t
	)

	if err := syscall.Stat(rootVolPath, &stat); err != nil {
		return err
	}
	if err := syscall.Stat(srcPath, &srcStat); err != nil {
		return err
	}
	// Change the source volume's ownership if it differs from the root
	// files that were just copied
	if stat.Uid != srcStat.Uid || stat.Gid != srcStat.Gid {
		if err := os.Chown(srcPath, int(stat.Uid), int(stat.Gid)); err != nil {
			return err
		}
	}
	return nil
}
