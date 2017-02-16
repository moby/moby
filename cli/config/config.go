package config

import (
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli/config/configfile"
	"github.com/docker/docker/pkg/homedir"
	"github.com/pkg/errors"
)

const (
	// ConfigFileName is the name of config file
	ConfigFileName   = "config.json"
	configFileDir    = "docker"
	dotconfigFileDir = "." + configFileDir
	oldConfigfile    = ".dockercfg"
)

var (
	readConfigDir  = os.Getenv("DOCKER_CONFIG")
	writeConfigDir = readConfigDir
)

// GetDir returns the specified directory inside the docker config directory.
// It handles legacy folders by checking their existence if the default one doesn't exists
func GetDir(name string) string {
	dir := filepath.Join(writeConfigDir, name)
	if readConfigDir == writeConfigDir {
		return dir
	}
	if _, err := os.Stat(dir); err != nil && os.IsNotExist(err) {
		legacyDir := filepath.Join(readConfigDir, name)
		if _, err := os.Stat(legacyDir); err == nil || os.IsExist(err) {
			return legacyDir
		}
	}
	return dir
}

// SetDir sets the directory the configuration file is stored in
func SetDir(dir string) {
	readConfigDir = dir
	writeConfigDir = dir
}

// NewConfigFile initializes an empty configuration file for the given filename 'fn'
func NewConfigFile(fn string) *configfile.ConfigFile {
	return &configfile.ConfigFile{
		AuthConfigs: make(map[string]types.AuthConfig),
		HTTPHeaders: make(map[string]string),
		Filename:    fn,
	}
}

// LegacyLoadFromReader is a convenience function that creates a ConfigFile object from
// a non-nested reader
func LegacyLoadFromReader(configData io.Reader) (*configfile.ConfigFile, error) {
	configFile := configfile.ConfigFile{
		AuthConfigs: make(map[string]types.AuthConfig),
	}
	err := configFile.LegacyLoadFromReader(configData)
	return &configFile, err
}

// LoadFromReader is a convenience function that creates a ConfigFile object from
// a reader
func LoadFromReader(configData io.Reader) (*configfile.ConfigFile, error) {
	configFile := configfile.ConfigFile{
		AuthConfigs: make(map[string]types.AuthConfig),
	}
	err := configFile.LoadFromReader(configData)
	return &configFile, err
}

// Load reads the configuration files in the given directory, and sets up
// the auth config information and returns values.
// FIXME: use the internal golang config parser
func Load() (*configfile.ConfigFile, error) {
	configFile := configfile.ConfigFile{
		AuthConfigs: make(map[string]types.AuthConfig),
		Filename:    filepath.Join(writeConfigDir, ConfigFileName),
	}

	// Try happy path first - latest config file
	if _, err := os.Stat(configFile.Filename); err == nil {
		file, err := os.Open(configFile.Filename)
		if err != nil {
			return &configFile, errors.Wrapf(err, "Error loading config file: %s", configFile.Filename)
		}
		defer file.Close()
		err = configFile.LoadFromReader(file)
		return &configFile, errors.Wrapf(err, "Error loading config file: %s", configFile.Filename)
	} else if os.IsNotExist(err) {
		legacyFilename := filepath.Join(readConfigDir, ConfigFileName)
		_, err := os.Stat(legacyFilename)
		if err != nil {
			if os.IsNotExist(err) {
				// Can't find latest config file so check for the old ones
				confFile := filepath.Join(homedir.Get(), oldConfigfile)
				if _, err := os.Stat(confFile); err != nil {
					return &configFile, nil //missing file is not an error
				}
				file, err := os.Open(confFile)
				if err != nil {
					return &configFile, errors.Wrapf(err, "Error loading config file: %s", confFile)
				}
				defer file.Close()
				err = configFile.LegacyLoadFromReader(file)
				if err != nil {
					return &configFile, errors.Wrapf(err, "Error loading config file: %s", confFile)
				}
				return &configFile, nil //missing file is not an error
			}
			return &configFile, errors.Wrapf(err, "Error loading config file: %s", legacyFilename)
		}
		file, err := os.Open(legacyFilename)
		if err != nil {
			return &configFile, errors.Wrapf(err, "Error loading config file: %s", legacyFilename)
		}
		defer file.Close()
		if err := configFile.LoadFromReader(file); err != nil {
			return &configFile, errors.Wrapf(err, "Error loading config file: %s", legacyFilename)
		}
		if err := configFile.Save(); err != nil {
			return &configFile, errors.Wrapf(err, "Error saving %s", legacyFilename)
		}
		return &configFile, errors.Errorf("configuration migrated from %s to %s", legacyFilename, configFile.Filename)
	} else if !os.IsNotExist(err) {
		// if file is there but we can't stat it for any reason other
		// than it doesn't exist then stop
		return &configFile, errors.Wrapf(err, "Error loading config file: %s", configFile.Filename)
	}

	if configFile.HTTPHeaders == nil {
		configFile.HTTPHeaders = map[string]string{}
	}
	return &configFile, nil
}
