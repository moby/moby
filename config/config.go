package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

// Where we store the config file
const CONFIGFILE = ".dockercfg"

var (
	ErrConfigFileMissing = errors.New("The Auth config file is missing")
)

type ConfigFile struct {
	Configs  map[string]interface{}
	rootPath string
}

func (config *ConfigFile) GetConfig(key string) (interface{}, error) {
	if config == nil {
		return nil, fmt.Errorf("config not initialised")
	}
	c, found := config.Configs[key]
	if found {
		return c, nil
	} else {
		return nil, fmt.Errorf("no config found for %s", key)
	}
}

func (config *ConfigFile) PutConfig(key string, data interface{}) error {
	if config == nil {
		return fmt.Errorf("config not initialised")
	}
	config.Configs[key] = data
	return SaveConfig(config)
}

// load up the auth config information and return values
// FIXME: use the internal golang config parser
func LoadConfig(rootPath string) (*ConfigFile, error) {
	configFile := ConfigFile{Configs: make(map[string]interface{}), rootPath: rootPath}
	confFile := path.Join(rootPath, CONFIGFILE)
	if _, err := os.Stat(confFile); err != nil {
		return &configFile, nil //missing file is not an error
	}
	b, err := ioutil.ReadFile(confFile)
	if err != nil {
		return &configFile, err
	}

	if err := json.Unmarshal(b, &configFile.Configs); err != nil {
		arr := strings.Split(string(b), "\n")
		if len(arr) < 2 {
			return &configFile, fmt.Errorf("The Auth config file is empty")
		}
		return &configFile, fmt.Errorf("Invalid Auth config file")
	}
	return &configFile, nil
}

// save the auth config
func SaveConfig(configFile *ConfigFile) error {
	confFile := path.Join(configFile.rootPath, CONFIGFILE)
	if len(configFile.Configs) == 0 {
		os.Remove(confFile)
		return nil
	}

	b, err := json.Marshal(configFile)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(confFile, b, 0600)
	if err != nil {
		return err
	}
	return nil
}
