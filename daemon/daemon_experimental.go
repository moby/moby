// +build experimental

package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/engine-api/types/container"
)

func setupRemappedRoot(config *Config) ([]idtools.IDMap, []idtools.IDMap, error) {
	if runtime.GOOS != "linux" && config.RemappedRoot != "" {
		return nil, nil, fmt.Errorf("User namespaces are only supported on Linux")
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
	config.Root = rootDir
	// the docker root metadata directory needs to have execute permissions for all users (o+x)
	// so that syscalls executing as non-root, operating on subdirectories of the graph root
	// (e.g. mounted layers of a container) can traverse this path.
	// The user namespace support will create subdirectories for the remapped root host uid:gid
	// pair owned by that same uid:gid pair for proper write access to those needed metadata and
	// layer content subtrees.
	if _, err := os.Stat(rootDir); err == nil {
		// root current exists; verify the access bits are correct by setting them
		if err = os.Chmod(rootDir, 0701); err != nil {
			return err
		}
	} else if os.IsNotExist(err) {
		// no root exists yet, create it 0701 with root:root ownership
		if err := os.MkdirAll(rootDir, 0701); err != nil {
			return err
		}
	}

	// if user namespaces are enabled we will create a subtree underneath the specified root
	// with any/all specified remapped root uid/gid options on the daemon creating
	// a new subdirectory with ownership set to the remapped uid/gid (so as to allow
	// `chdir()` to work for containers namespaced to that uid/gid)
	if config.RemappedRoot != "" {
		config.Root = filepath.Join(rootDir, fmt.Sprintf("%d.%d", rootUID, rootGID))
		logrus.Debugf("Creating user namespaced daemon root: %s", config.Root)
		// Create the root directory if it doesn't exists
		if err := idtools.MkdirAllAs(config.Root, 0700, rootUID, rootGID); err != nil {
			return fmt.Errorf("Cannot create daemon root: %s: %v", config.Root, err)
		}
	}
	return nil
}

func (daemon *Daemon) verifyExperimentalContainerSettings(hostConfig *container.HostConfig, config *container.Config) ([]string, error) {
	if hostConfig.Privileged && daemon.configStore.RemappedRoot != "" {
		return nil, fmt.Errorf("Privileged mode is incompatible with user namespace mappings")
	}
	return nil, nil
}
