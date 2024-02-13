package homedir // import "github.com/docker/docker/pkg/homedir"

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// GetRuntimeDir returns XDG_RUNTIME_DIR.
// XDG_RUNTIME_DIR is typically configured via pam_systemd.
// GetRuntimeDir returns non-nil error if XDG_RUNTIME_DIR is not set.
//
// See also https://standards.freedesktop.org/basedir-spec/latest/ar01s03.html
func GetRuntimeDir() (string, error) {
	if xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR"); xdgRuntimeDir != "" {
		return xdgRuntimeDir, nil
	}
	return "", errors.New("could not get XDG_RUNTIME_DIR")
}

// StickRuntimeDirContents sets the sticky bit on files that are under
// XDG_RUNTIME_DIR, so that the files won't be periodically removed by the system.
//
// StickyRuntimeDir returns slice of sticked files.
// StickyRuntimeDir returns nil error if XDG_RUNTIME_DIR is not set.
//
// See also https://standards.freedesktop.org/basedir-spec/latest/ar01s03.html
func StickRuntimeDirContents(files []string) ([]string, error) {
	runtimeDir, err := GetRuntimeDir()
	if err != nil {
		// ignore error if runtimeDir is empty
		return nil, nil
	}
	runtimeDir, err = filepath.Abs(runtimeDir)
	if err != nil {
		return nil, err
	}
	var sticked []string
	for _, f := range files {
		f, err = filepath.Abs(f)
		if err != nil {
			return sticked, err
		}
		if strings.HasPrefix(f, runtimeDir+"/") {
			if err = stick(f); err != nil {
				return sticked, err
			}
			sticked = append(sticked, f)
		}
	}
	return sticked, nil
}

func stick(f string) error {
	st, err := os.Stat(f)
	if err != nil {
		return err
	}
	m := st.Mode()
	m |= os.ModeSticky
	return os.Chmod(f, m)
}

// GetDataHome returns XDG_DATA_HOME.
// GetDataHome returns $HOME/.local/share and nil error if XDG_DATA_HOME is not set.
// If HOME and XDG_DATA_HOME are not set, getpwent(3) is consulted to determine the users home directory.
//
// See also https://standards.freedesktop.org/basedir-spec/latest/ar01s03.html
func GetDataHome() (string, error) {
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		return xdgDataHome, nil
	}
	home := Get()
	if home == "" {
		return "", errors.New("could not get either XDG_DATA_HOME or HOME")
	}
	return filepath.Join(home, ".local", "share"), nil
}

// GetConfigHome returns XDG_CONFIG_HOME.
// GetConfigHome returns $HOME/.config and nil error if XDG_CONFIG_HOME is not set.
// If HOME and XDG_CONFIG_HOME are not set, getpwent(3) is consulted to determine the users home directory.
//
// See also https://standards.freedesktop.org/basedir-spec/latest/ar01s03.html
func GetConfigHome() (string, error) {
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return xdgConfigHome, nil
	}
	home := Get()
	if home == "" {
		return "", errors.New("could not get either XDG_CONFIG_HOME or HOME")
	}
	return filepath.Join(home, ".config"), nil
}

// GetLibHome returns $HOME/.local/lib
// If HOME is not set, getpwent(3) is consulted to determine the users home directory.
func GetLibHome() (string, error) {
	home := Get()
	if home == "" {
		return "", errors.New("could not get HOME")
	}
	return filepath.Join(home, ".local/lib"), nil
}
