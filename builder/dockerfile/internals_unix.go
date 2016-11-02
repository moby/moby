// +build !windows

package dockerfile

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
)

const (
	// /run is used instead of /run/secrets to keep /run/secrets
	// out of the layer upon commit
	secretContainerPath = "/run"
)

// normaliseDest normalises the destination of a COPY/ADD command in a
// platform semantically consistent way.
func normaliseDest(cmdName, workingDir, requested string) (string, error) {
	dest := filepath.FromSlash(requested)
	endsInSlash := strings.HasSuffix(requested, string(os.PathSeparator))
	if !system.IsAbs(requested) {
		dest = filepath.Join(string(os.PathSeparator), filepath.FromSlash(workingDir), dest)
		// Make sure we preserve any trailing slash
		if endsInSlash {
			dest += string(os.PathSeparator)
		}
	}
	return dest, nil
}

func containsWildcards(name string) bool {
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '\\' {
			i++
		} else if ch == '*' || ch == '?' || ch == '[' {
			return true
		}
	}
	return false
}

// setupSecretMount is used to setup a tmpfs filesystem as a host mount
func setupSecretMount() (*mounttypes.Mount, error) {
	tempDir, err := ioutil.TempDir("", "docker-build-secrets-")
	if err != nil {
		return nil, errors.Wrap(err, "unable to create temp directory for secrets")
	}
	if err := mount.Mount("tmpfs", tempDir, "tmpfs", "nodev,nosuid,noexec"); err != nil {
		return nil, errors.Wrap(err, "unable to setup build secret mount")
	}

	return &mounttypes.Mount{
		Type:   mounttypes.TypeBind,
		Source: tempDir,
		Target: secretContainerPath,
	}, nil
}

// cleanupSecretMount unmounts the secret mount and removes the directory
func cleanupSecretMount(dir string) error {
	if err := mount.ForceUnmount(dir); err != nil {
		return errors.Wrap(err, "unable to cleanup build secret mount")
	}

	if err := os.RemoveAll(dir); err != nil {
		return err
	}

	return nil
}
