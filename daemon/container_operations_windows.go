package daemon

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/libnetwork"
	"github.com/pkg/errors"
)

func (daemon *Daemon) setupLinkedContainers(container *container.Container) ([]string, error) {
	return nil, nil
}

func (daemon *Daemon) setupConfigDir(c *container.Container) (setupErr error) {
	if len(c.ConfigReferences) == 0 {
		return nil
	}

	localPath := c.ConfigsDirPath()
	logrus.Debugf("configs: setting up config dir: %s", localPath)

	// create local config root
	if err := system.MkdirAllWithACL(localPath, 0); err != nil {
		return errors.Wrap(err, "error creating config dir")
	}

	defer func() {
		if setupErr != nil {
			if err := os.RemoveAll(localPath); err != nil {
				logrus.Errorf("error cleaning up config dir: %s", err)
			}
		}
	}()

	if c.DependencyStore == nil {
		return fmt.Errorf("config store is not initialized")
	}

	for _, configRef := range c.ConfigReferences {
		// TODO (ehazlett): use type switch when more are supported
		if configRef.File == nil {
			logrus.Error("config target type is not a file target")
			continue
		}

		fPath := c.ConfigFilePath(*configRef)

		log := logrus.WithFields(logrus.Fields{"name": configRef.File.Name, "path": fPath})

		log.Debug("injecting config")
		config := c.DependencyStore.Configs().Get(configRef.ConfigID)
		if config == nil {
			return fmt.Errorf("unable to get config from config store")
		}
		if err := ioutil.WriteFile(fPath, config.Spec.Data, configRef.File.Mode); err != nil {
			return errors.Wrap(err, "error injecting config")
		}
	}

	return nil
}

// getSize returns real size & virtual size
func (daemon *Daemon) getSize(containerID string) (int64, int64) {
	// TODO Windows
	return 0, 0
}

func (daemon *Daemon) setupIpcDirs(container *container.Container) error {
	return nil
}

// TODO Windows: Fix Post-TP5. This is a hack to allow docker cp to work
// against containers which have volumes. You will still be able to cp
// to somewhere on the container drive, but not to any mounted volumes
// inside the container. Without this fix, docker cp is broken to any
// container which has a volume, regardless of where the file is inside the
// container.
func (daemon *Daemon) mountVolumes(container *container.Container) error {
	return nil
}

func detachMounted(path string) error {
	return nil
}

func (daemon *Daemon) setupSecretDir(c *container.Container) (setupErr error) {
	if len(c.SecretReferences) == 0 {
		return nil
	}

	localMountPath := c.SecretMountPath()
	logrus.Debugf("secrets: setting up secret dir: %s", localMountPath)

	// create local secret root
	if err := system.MkdirAllWithACL(localMountPath, 0); err != nil {
		return errors.Wrap(err, "error creating secret local directory")
	}

	defer func() {
		if setupErr != nil {
			if err := os.RemoveAll(localMountPath); err != nil {
				logrus.Errorf("error cleaning up secret mount: %s", err)
			}
		}
	}()

	if c.DependencyStore == nil {
		return fmt.Errorf("secret store is not initialized")
	}

	for _, s := range c.SecretReferences {
		// TODO (ehazlett): use type switch when more are supported
		if s.File == nil {
			logrus.Error("secret target type is not a file target")
			continue
		}

		// secrets are created in the SecretMountPath on the host, at a
		// single level
		fPath := c.SecretFilePath(*s)
		logrus.WithFields(logrus.Fields{
			"name": s.File.Name,
			"path": fPath,
		}).Debug("injecting secret")
		secret := c.DependencyStore.Secrets().Get(s.SecretID)
		if secret == nil {
			return fmt.Errorf("unable to get secret from secret store")
		}
		if err := ioutil.WriteFile(fPath, secret.Spec.Data, s.File.Mode); err != nil {
			return errors.Wrap(err, "error injecting secret")
		}
	}

	return nil
}

func killProcessDirectly(container *container.Container) error {
	return nil
}

func isLinkable(child *container.Container) bool {
	return false
}

func enableIPOnPredefinedNetwork() bool {
	return true
}

func (daemon *Daemon) isNetworkHotPluggable() bool {
	return false
}

func setupPathsAndSandboxOptions(container *container.Container, sboxOptions *[]libnetwork.SandboxOption) error {
	return nil
}

func initializeNetworkingPaths(container *container.Container, nc *container.Container) {
	container.NetworkSharedContainerID = nc.ID
}
