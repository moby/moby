package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/docker/docker/api"
)

var (
	configFileValues  map[string]interface{}
	defaultStorageDir string
	defaultGetter     = DefaultGetter{}
	dockerConfigPath  string
	dockerCertPath    string
	dockerTlsVerify   bool
	configFileHost    string
)

type DefaultGetter struct {
	configFileValues map[string]interface{}
}

type ConfigHierarchy struct {
	EnvVar  string
	JsonKey string
	Default interface{}
}

// The hierarchy of configuration options flows like
// this, in order of most preferred to least preferred:
//
// Command Line Flags => Environment Variables => defaults.json => Hardcoded Defaults
func getHomeDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}

func getConfigPath() string {
	// This is a "special case" since we
	// need to know about it to check the other
	// defaut values.
	envVal := os.Getenv("DOCKER_CONFIG_PATH")
	if envVal == "" {
		return defaultStorageDir
	} else {
		return envVal
	}
}

func (d *DefaultGetter) GetCertPath() string {
	return d.getConfigValue(configFileValues, ConfigHierarchy{
		EnvVar:  "DOCKER_CERT_PATH",
		JsonKey: "CertPath",
		Default: defaultStorageDir,
	}).(string)
}

func (d *DefaultGetter) GetTlsVerify() bool {
	configVal := d.getConfigValue(configFileValues, ConfigHierarchy{
		EnvVar:  "DOCKER_TLS_VERIFY",
		JsonKey: "TlsVerify",
		Default: "false",
	})
	if boolVal, ok := configVal.(bool); ok {
		return boolVal
	}
	if stringVal, ok := configVal.(string); ok {
		val, err := strconv.ParseBool(stringVal)
		if err != nil {
			log.Fatal("Error parsing TlsVerify / DOCKER_TLS_VERIFY value: %s", err)
		}
		return val
	}
	log.Fatal("Unrecognized type for TlsVerify value in config file")
	return false
}

func (d *DefaultGetter) GetHost() string {
	return d.getConfigValue(configFileValues, ConfigHierarchy{
		EnvVar:  "DOCKER_HOST",
		JsonKey: "Host",
		Default: fmt.Sprintf("unix://%s", api.DEFAULTUNIXSOCKET),
	}).(string)
}

func (d *DefaultGetter) readConfigFile(dockerConfigPath string) {
	var (
		cfv map[string]interface{}
	)
	configFilePath := filepath.Join(dockerConfigPath, "defaults.json")
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		d.configFileValues = nil
		return
	}
	data, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		switch err {
		case os.ErrPermission:
			log.Fatalf("Error reading %s: Insufficient permissions", configFilePath)
		default:
			log.Fatalf("Unrecognized error reading %s: %s", configFilePath, err)
		}
	}
	if json.Unmarshal(data, &cfv); err != nil {
		log.Fatalf("Error unmarshalling %s: %s", configFilePath, err)
	}
	d.configFileValues = cfv
}

func (d *DefaultGetter) getConfigValue(configFileValues map[string]interface{}, c ConfigHierarchy) interface{} {
	envVal := os.Getenv(c.EnvVar)
	if envVal == "" {
		if d.configFileValues != nil {
			if d.configFileValues[c.JsonKey] != nil {
				return d.configFileValues[c.JsonKey]
			}
		}
	} else {
		return envVal
	}
	return c.Default
}
