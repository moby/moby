package daemon // import "github.com/docker/docker/daemon"

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/docker/docker/container"
	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	if err := system.MkdirAllWithACL(localPath, 0, system.SddlAdministratorsLocalSystem); err != nil {
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
			// Runtime configs are not mounted into the container, but they're
			// a valid type of config so we should not error when we encounter
			// one.
			if configRef.Runtime == nil {
				logrus.Error("config target type is not a file or runtime target")
			}
			// However, in any case, this isn't a file config, so we have no
			// further work to do
			continue
		}

		fPath, err := c.ConfigFilePath(*configRef)
		if err != nil {
			return errors.Wrap(err, "error getting config file path for container")
		}
		log := logrus.WithFields(logrus.Fields{"name": configRef.File.Name, "path": fPath})

		log.Debug("injecting config")
		config, err := c.DependencyStore.Configs().Get(configRef.ConfigID)
		if err != nil {
			return errors.Wrap(err, "unable to get config from config store")
		}
		if err := ioutil.WriteFile(fPath, config.Spec.Data, configRef.File.Mode); err != nil {
			return errors.Wrap(err, "error injecting config")
		}
	}

	return nil
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

func (daemon *Daemon) setupSecretDir(c *container.Container) (setupErr error) {
	if len(c.SecretReferences) == 0 {
		return nil
	}

	localMountPath, err := c.SecretMountPath()
	if err != nil {
		return err
	}
	logrus.Debugf("secrets: setting up secret dir: %s", localMountPath)

	// create local secret root
	if err := system.MkdirAllWithACL(localMountPath, 0, system.SddlAdministratorsLocalSystem); err != nil {
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
		fPath, err := c.SecretFilePath(*s)
		if err != nil {
			return err
		}
		logrus.WithFields(logrus.Fields{
			"name": s.File.Name,
			"path": fPath,
		}).Debug("injecting secret")
		secret, err := c.DependencyStore.Secrets().Get(s.SecretID)
		if err != nil {
			return errors.Wrap(err, "unable to get secret from secret store")
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

// serviceDiscoveryOnDefaultNetwork indicates if service discovery is supported on the default network
func serviceDiscoveryOnDefaultNetwork() bool {
	return true
}

func (daemon *Daemon) setupPathsAndSandboxOptions(container *container.Container, sboxOptions *[]libnetwork.SandboxOption) error {
	return nil
}

func (daemon *Daemon) initializeNetworkingPaths(container *container.Container, nc *container.Container) error {

	if nc.HostConfig.Isolation.IsHyperV() {
		return fmt.Errorf("sharing of hyperv containers network is not supported")
	}

	container.NetworkSharedContainerID = nc.ID

	if nc.NetworkSettings != nil {
		for n := range nc.NetworkSettings.Networks {
			sn, err := daemon.FindNetwork(n)
			if err != nil {
				continue
			}

			ep, err := getEndpointInNetwork(nc.Name, sn)
			if err != nil {
				continue
			}

			data, err := ep.DriverInfo()
			if err != nil {
				continue
			}

			if data["GW_INFO"] != nil {
				gwInfo := data["GW_INFO"].(map[string]interface{})
				if gwInfo["hnsid"] != nil {
					container.SharedEndpointList = append(container.SharedEndpointList, gwInfo["hnsid"].(string))
				}
			}

			if data["hnsid"] != nil {
				container.SharedEndpointList = append(container.SharedEndpointList, data["hnsid"].(string))
			}
		}
	}

	return nil
}
