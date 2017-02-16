package xdg

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/homedir"
)

const (
	defaultConfig          = ".config"
	defaultGlobalConfigDir = "/etc/xdg"
)

// GetConfigFile returns the path of the specified file following xdg base directory specifications
func GetConfigFile(filename string) (string, error) {
	return getFileNameFromDirs(filename, getConfigDirs())
}
func getConfigDirs() []string {
	dirs := []string{}
	xdgHomeConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgHomeConfig != "" {
		dirs = append(dirs, xdgHomeConfig)
	} else {
		dirs = append(dirs, filepath.Join(homedir.Get(), defaultConfig))
	}
	xdgConfigDirs := os.Getenv("XDG_CONFIG_DIRS")
	if xdgConfigDirs != "" {
		configDirs := strings.Split(xdgConfigDirs, ":")
		for _, configDir := range configDirs {
			dirs = append(dirs, configDir)
		}
	} else {
		dirs = append(dirs, defaultGlobalConfigDir)
	}
	return dirs
}

func getFileNameFromDirs(filename string, dirs []string) (string, error) {
	defaultDir := dirs[0]
	for _, dir := range dirs {
		fileLoc := filepath.Join(dir, filename)
		if _, err := os.Stat(fileLoc); err != nil {
			continue
		}
		return fileLoc, nil
	}
	return filepath.Join(defaultDir, filename), os.ErrNotExist
}
