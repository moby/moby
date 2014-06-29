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
	var (
		volPath   string
		mountPath string
		mountRW   bool
		hostPath  string
	)
	volumesFrom := container.hostConfig.VolumesFrom
	if len(volumesFrom) > 0 {
		for _, volSpec := range volumesFrom {
			volSpec := strings.SplitN(volSpec, ":", 4)

			mountRW = true
			switch len(volSpec) {
			case 2:
				switch volSpec[1] {
				case "rw":
				case "ro":
					mountRW = false
				default:
					volPath = volSpec[1]
				}
			case 3:
				switch volSpec[2] {
				case "rw":
				case "ro":
					mountRW = false
				default:
					mountPath = volSpec[2]
				}
				volPath = volSpec[1]
			case 4:
				switch volSpec[3] {
				case "rw":
				case "ro":
					mountRW = false
				default:
					return fmt.Errorf("Malformed volumes-from specification: %s", volSpec)
				}
				volPath = volSpec[1]
				mountPath = volSpec[2]
			}

			c := container.daemon.Get(volSpec[0])

			if volPath != "" {
				// Check if volume ID or container ID was used
				if c != nil {
					hostPath = c.Volumes[volPath]
				} else {
					hostPath = container.daemon.Volumes().Root + "/" + volSpec[0]
				}

				if _, err := os.Stat(hostPath); err != nil && os.IsNotExist(err) {
					return fmt.Errorf("No such file or dir: %v", hostPath)
				}

				if mountPath != "" {
					container.Volumes[mountPath] = hostPath
					container.VolumesRW[mountPath] = mountRW
				} else {
					container.Volumes[volPath] = hostPath
					container.VolumesRW[volPath] = mountRW
				}
			} else {
				for volPath, id := range c.Volumes {
					if _, exists := container.Volumes[volPath]; exists {
						continue
					}

					_, err := os.Stat(id)
					if err != nil {
						return err
					}

					container.Volumes[volPath] = id
					if isRw, exists := c.VolumesRW[volPath]; exists {
						container.VolumesRW[volPath] = isRw && mountRW
					}
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

func createIfNotExists(destination string, isDir bool) error {
	if _, err := os.Stat(destination); err != nil && os.IsNotExist(err) {
		if isDir {
			if err := os.MkdirAll(destination, 0755); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
				return err
			}

			f, err := os.OpenFile(destination, os.O_CREATE, 0755)
			if err != nil {
				return err
			}
			f.Close()
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
		destination string
		isBindMount bool
		volIsDir    = true

		srcRW = false
	)

	// If an external bind is defined for this volume, use that as a source
	if bindMap, exists := binds[volPath]; exists {
		isBindMount = true
		destination = bindMap.SrcPath

		if !filepath.IsAbs(destination) {
			return fmt.Errorf("%s must be an absolute path", destination)
		}

		if strings.ToLower(bindMap.Mode) == "rw" {
			srcRW = true
		}

		if stat, err := os.Stat(bindMap.SrcPath); err != nil {
			return err
		} else {
			volIsDir = stat.IsDir()
		}
	} else {
		// Do not pass a container as the parameter for the volume creation.
		// The graph driver using the container's information ( Image ) to
		// create the parent.
		c, err := container.daemon.volumes.Create(nil, "", "", "", "", nil, nil)
		if err != nil {
			return err
		}

		destination, err = volumesDriver.Get(c.ID, "")
		if err != nil {
			return fmt.Errorf("Driver %s failed to get volume rootfs %s: %s", volumesDriver, c.ID, err)
		}

		srcRW = true
	}

	if p, err := filepath.EvalSymlinks(destination); err != nil {
		return err
	} else {
		destination = p
	}

	// Create the mountpoint
	source, err := symlink.FollowSymlinkInScope(filepath.Join(container.basefs, volPath), container.basefs)
	if err != nil {
		return err
	}

	newVolPath, err := filepath.Rel(container.basefs, source)
	if err != nil {
		return err
	}
	newVolPath = "/" + newVolPath

	if volPath != newVolPath {
		delete(container.Volumes, volPath)
		delete(container.VolumesRW, volPath)
	}

	container.Volumes[volPath] = destination
	container.VolumesRW[volPath] = srcRW

	if err := createIfNotExists(source, volIsDir); err != nil {
		return err
	}

	// Do not copy or change permissions if we are mounting from the host
	if srcRW && !isBindMount {
		if err := copyExistingContents(source, destination); err != nil {
			return err
		}
	}
	return nil
}

func copyExistingContents(source, destination string) error {
	volList, err := ioutil.ReadDir(source)
	if err != nil {
		return err
	}

	if len(volList) > 0 {
		srcList, err := ioutil.ReadDir(destination)
		if err != nil {
			return err
		}

		if len(srcList) == 0 {
			// If the source volume is empty copy files from the root into the volume
			if err := archive.CopyWithTar(source, destination); err != nil {
				return err
			}
		}
	}

	return copyOwnership(source, destination)
}

// copyOwnership copies the permissions and uid:gid of the source file
// into the destination file
func copyOwnership(source, destination string) error {
	var stat syscall.Stat_t

	if err := syscall.Stat(source, &stat); err != nil {
		return err
	}

	if err := os.Chown(destination, int(stat.Uid), int(stat.Gid)); err != nil {
		return err
	}

	return os.Chmod(destination, os.FileMode(stat.Mode))
}
