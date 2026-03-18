package homedir

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// GetRuntimeDir returns [XDG_RUNTIME_DIR]. It returns a non-nil error if
// XDG_RUNTIME_DIR is not set. XDG_RUNTIME_DIR is typically configured via
// [pam_systemd].
//
// [XDG_RUNTIME_DIR]: https://specifications.freedesktop.org/basedir/0.8/#variables
// [pam_systemd]: https://man7.org/linux/man-pages/man8/pam_systemd.8.html
func GetRuntimeDir() (string, error) {
	if xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR"); xdgRuntimeDir != "" {
		return xdgRuntimeDir, nil
	}
	return "", errors.New("could not get XDG_RUNTIME_DIR")
}

// StickRuntimeDirContents sets the sticky bit on files that are under
// [XDG_RUNTIME_DIR], so that the files won't be periodically removed by the
// system.
//
// It returns a slice of sticked files as absolute paths. The list of files may
// be empty (nil) if XDG_RUNTIME_DIR is not set, in which case no error is returned.
// StickyRuntimeDir produces an error when failing to resolve the absolute path
// for the returned files.
//
// [XDG_RUNTIME_DIR]: https://specifications.freedesktop.org/basedir/0.8/#variables
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

// GetDataHome returns [XDG_DATA_HOME] or $HOME/.local/share and a nil error if
// [XDG_DATA_HOME] is not set. If neither HOME nor XDG_DATA_HOME are set,
// [getpwent(3)] is consulted to determine the users home directory.
//
// [XDG_DATA_HOME]: https://specifications.freedesktop.org/basedir/0.8/#variables
// [getpwent(3)]: https://man7.org/linux/man-pages/man3/getpwent.3.html
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

// GetConfigHome returns [XDG_CONFIG_HOME] or $HOME/.config and a nil error if
// XDG_CONFIG_HOME is not set. If neither HOME nor XDG_CONFIG_HOME are set,
// [getpwent(3)] is consulted to determine the users home directory.
//
// [XDG_CONFIG_HOME]: https://specifications.freedesktop.org/basedir/0.8/#variables
// [getpwent(3)]: https://man7.org/linux/man-pages/man3/getpwent.3.html
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

// GetLibHome returns $HOME/.local/lib. If HOME is not set, [getpwent(3)] is
// consulted to determine the users home directory.
//
// [getpwent(3)]: https://man7.org/linux/man-pages/man3/getpwent.3.html
func GetLibHome() (string, error) {
	home := Get()
	if home == "" {
		return "", errors.New("could not get HOME")
	}
	return filepath.Join(home, ".local/lib"), nil
}

// GetLibexecHome returns $HOME/.local/libexec. If HOME is not set,
// [getpwent(3)] is consulted to determine the users home directory.
//
// [getpwent(3)]: https://man7.org/linux/man-pages/man3/getpwent.3.html
func GetLibexecHome() (string, error) {
	home := Get()
	if home == "" {
		return "", errors.New("could not get HOME")
	}
	return filepath.Join(home, ".local/libexec"), nil
}
