// +build experimental

package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/directory"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/runconfig"
)

func setupRemappedRoot(config *Config) ([]idtools.IDMap, []idtools.IDMap, error) {
	if config.ExecDriver != "native" && config.RemappedRoot != "" {
		return nil, nil, fmt.Errorf("User namespace remapping is only supported with the native execdriver")
	}
	if runtime.GOOS == "windows" && config.RemappedRoot != "" {
		return nil, nil, fmt.Errorf("User namespaces are not supported on Windows")
	}

	// if the daemon was started with remapped root option, parse
	// the config option to the int uid,gid values
	var (
		uidMaps, gidMaps []idtools.IDMap
	)
	if config.RemappedRoot != "" {
		username, groupname, err := parseRemappedRoot(config.RemappedRoot)
		if err != nil {
			return nil, nil, err
		}
		if username == "root" {
			// Cannot setup user namespaces with a 1-to-1 mapping; "--root=0:0" is a no-op
			// effectively
			logrus.Warnf("User namespaces: root cannot be remapped with itself; user namespaces are OFF")
			return uidMaps, gidMaps, nil
		}
		logrus.Infof("User namespaces: ID ranges will be mapped to subuid/subgid ranges of: %s:%s", username, groupname)
		// update remapped root setting now that we have resolved them to actual names
		config.RemappedRoot = fmt.Sprintf("%s:%s", username, groupname)

		uidMaps, gidMaps, err = idtools.CreateIDMappings(username, groupname)
		if err != nil {
			return nil, nil, fmt.Errorf("Can't create ID mappings: %v", err)
		}
	}
	return uidMaps, gidMaps, nil
}

func setupDaemonRoot(config *Config, rootDir string, rootUID, rootGID int) error {
	// the main docker root needs to be accessible by all users, as user namespace support
	// will create subdirectories owned by either a) the real system root (when no remapping
	// is setup) or b) the remapped root host ID (when --root=uid:gid is used)
	// for "first time" users of user namespaces, we need to migrate the current directory
	// contents to the "0.0" (root == root "namespace" daemon root)
	nsRoot := "0.0"
	if _, err := os.Stat(rootDir); err == nil {
		// root current exists; we need to check for a prior migration
		if _, err := os.Stat(filepath.Join(rootDir, nsRoot)); err != nil && os.IsNotExist(err) {
			// need to migrate current root to "0.0" subroot
			// 1. create non-usernamespaced root as "0.0"
			if err := os.Mkdir(filepath.Join(rootDir, nsRoot), 0700); err != nil {
				return fmt.Errorf("Cannot create daemon root %q: %v", filepath.Join(rootDir, nsRoot), err)
			}
			// 2. move current root content to "0.0" new subroot
			if err := directory.MoveToSubdir(rootDir, nsRoot); err != nil {
				return fmt.Errorf("Cannot migrate current daemon root %q for user namespaces: %v", rootDir, err)
			}
			// 3. chmod outer root to 755
			if chmodErr := os.Chmod(rootDir, 0755); chmodErr != nil {
				return chmodErr
			}
		}
	} else if os.IsNotExist(err) {
		// no root exists yet, create it 0755 with root:root ownership
		if err := os.MkdirAll(rootDir, 0755); err != nil {
			return err
		}
		// create the "0.0" subroot (so no future "migration" happens of the root)
		if err := os.Mkdir(filepath.Join(rootDir, nsRoot), 0700); err != nil {
			return err
		}
	}

	// for user namespaces we will create a subtree underneath the specified root
	// with any/all specified remapped root uid/gid options on the daemon creating
	// a new subdirectory with ownership set to the remapped uid/gid (so as to allow
	// `chdir()` to work for containers namespaced to that uid/gid)
	if config.RemappedRoot != "" {
		nsRoot = fmt.Sprintf("%d.%d", rootUID, rootGID)
	}
	config.Root = filepath.Join(rootDir, nsRoot)
	logrus.Debugf("Creating actual daemon root: %s", config.Root)

	// Create the root directory if it doesn't exists
	if err := idtools.MkdirAllAs(config.Root, 0700, rootUID, rootGID); err != nil {
		return fmt.Errorf("Cannot create daemon root: %s: %v", config.Root, err)
	}
	return nil
}

func (daemon *Daemon) verifyExperimentalContainerSettings(hostConfig *runconfig.HostConfig, config *runconfig.Config) ([]string, error) {
	if hostConfig.Privileged && daemon.config().RemappedRoot != "" {
		return nil, fmt.Errorf("Privileged mode is incompatible with user namespace mappings")
	}
	return nil, nil
}
