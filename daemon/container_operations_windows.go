// +build windows

package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/cloudflare/cfssl/log"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/libnetwork"
	"github.com/pkg/errors"
)

func (daemon *Daemon) setupLinkedContainers(container *container.Container) ([]string, error) {
	return nil, nil
}

// getSize returns real size & virtual size
func (daemon *Daemon) getSize(container *container.Container) (int64, int64) {
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

	defer func() {
		if setupErr != nil {
			if err := os.RemoveAll(localMountPath); err != nil {
				log.Errorf("error cleaning up secret mount: %s", err)
			}
		}
	}()

	// create secrets root
	system.MkdirAll(localMountPath, 0)

	for _, s := range c.SecretReferences {
		if c.SecretStore == nil {
			return fmt.Errorf("secret store is not initialized")
		}

		// TODO (ehazlett): use type switch when more are supported
		if s.File == nil {
			return fmt.Errorf("secret target type is not a file target")
		}

		targetPath := filepath.Clean(s.File.Name)
		// ensure that the target is a filename only; no paths allowed
		if targetPath != filepath.Base(targetPath) {
			return fmt.Errorf("error creating secret: secret must not be a path")
		}

		fPath := filepath.Join(localMountPath, targetPath)
		if err := system.MkdirAll(filepath.Dir(fPath), 0); err != nil {
			return errors.Wrap(err, "error creating secret mount path")
		}

		logrus.WithFields(logrus.Fields{
			"name": s.File.Name,
			"path": fPath,
		}).Debug("injecting secret")
		secret := c.SecretStore.Get(s.SecretID)
		if secret == nil {
			return fmt.Errorf("unable to get secret from secret store")
		}
		if err := system.WriteFileTemporary(fPath, secret.Spec.Data, s.File.Mode); err != nil {
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
}
