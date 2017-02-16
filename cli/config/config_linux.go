package config

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/cli/config/xdg"
	"github.com/docker/docker/pkg/homedir"
)

func init() {
	if readConfigDir == "" {
		xdgConfigDir, err := xdg.GetConfigFile(configFileDir)
		if err != nil {
			legacyConfigDir := filepath.Join(homedir.Get(), dotconfigFileDir)
			//_, err := os.Stat(filepath.Join(legacyConfigDir, ConfigFileName))
			_, err := os.Stat(legacyConfigDir)
			if err == nil || os.IsExist(err) {
				readConfigDir = legacyConfigDir
			}
		} else {
			_, err := os.Stat(filepath.Join(xdgConfigDir, ConfigFileName))
			if err == nil || os.IsExist(err) {
				readConfigDir = xdgConfigDir
			}
		}
		writeConfigDir = xdgConfigDir
	}
}
