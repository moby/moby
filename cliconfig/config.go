package cliconfig

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/homedir"
)

const (
	// Where we store the config file
	CONFIGFILE     = "config.json"
	OLD_CONFIGFILE = ".dockercfg"

	// This constant is only used for really old config files when the
	// URL wasn't saved as part of the config file and it was just
	// assumed to be this value.
	DEFAULT_INDEXSERVER = "https://index.docker.io/v1/"
)

var (
	ErrConfigFileMissing = errors.New("The Auth config file is missing")
)

// Registry Auth Info
type AuthConfig struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Auth          string `json:"auth"`
	Email         string `json:"email"`
	ServerAddress string `json:"serveraddress,omitempty"`
}

// ~/.docker/config.json file info
type ConfigFile struct {
	AuthConfigs map[string]AuthConfig `json:"auths"`
	HttpHeaders map[string]string     `json:"HttpHeaders,omitempty"`
	filename    string                // Note: not serialized - for internal use only
}

func NewConfigFile(fn string) *ConfigFile {
	return &ConfigFile{
		AuthConfigs: make(map[string]AuthConfig),
		HttpHeaders: make(map[string]string),
		filename:    fn,
	}
}

// load up the auth config information and return values
// FIXME: use the internal golang config parser
func Load(configDir string) (*ConfigFile, error) {
	if configDir == "" {
		configDir = filepath.Join(homedir.Get(), ".docker")
	}

	configFile := ConfigFile{
		AuthConfigs: make(map[string]AuthConfig),
		filename:    filepath.Join(configDir, CONFIGFILE),
	}

	// Try happy path first - latest config file
	if _, err := os.Stat(configFile.filename); err == nil {
		file, err := os.Open(configFile.filename)
		if err != nil {
			return &configFile, err
		}
		defer file.Close()

		if err := json.NewDecoder(file).Decode(&configFile); err != nil {
			return &configFile, err
		}

		for addr, ac := range configFile.AuthConfigs {
			ac.Username, ac.Password, err = DecodeAuth(ac.Auth)
			if err != nil {
				return &configFile, err
			}
			ac.Auth = ""
			ac.ServerAddress = addr
			configFile.AuthConfigs[addr] = ac
		}

		return &configFile, nil
	} else if !os.IsNotExist(err) {
		// if file is there but we can't stat it for any reason other
		// than it doesn't exist then stop
		return &configFile, err
	}

	// Can't find latest config file so check for the old one
	confFile := filepath.Join(homedir.Get(), OLD_CONFIGFILE)

	if _, err := os.Stat(confFile); err != nil {
		return &configFile, nil //missing file is not an error
	}

	b, err := ioutil.ReadFile(confFile)
	if err != nil {
		return &configFile, err
	}

	if err := json.Unmarshal(b, &configFile.AuthConfigs); err != nil {
		arr := strings.Split(string(b), "\n")
		if len(arr) < 2 {
			return &configFile, fmt.Errorf("The Auth config file is empty")
		}
		authConfig := AuthConfig{}
		origAuth := strings.Split(arr[0], " = ")
		if len(origAuth) != 2 {
			return &configFile, fmt.Errorf("Invalid Auth config file")
		}
		authConfig.Username, authConfig.Password, err = DecodeAuth(origAuth[1])
		if err != nil {
			return &configFile, err
		}
		origEmail := strings.Split(arr[1], " = ")
		if len(origEmail) != 2 {
			return &configFile, fmt.Errorf("Invalid Auth config file")
		}
		authConfig.Email = origEmail[1]
		authConfig.ServerAddress = DEFAULT_INDEXSERVER
		configFile.AuthConfigs[DEFAULT_INDEXSERVER] = authConfig
	} else {
		for k, authConfig := range configFile.AuthConfigs {
			authConfig.Username, authConfig.Password, err = DecodeAuth(authConfig.Auth)
			if err != nil {
				return &configFile, err
			}
			authConfig.Auth = ""
			authConfig.ServerAddress = k
			configFile.AuthConfigs[k] = authConfig
		}
	}
	return &configFile, nil
}

func (configFile *ConfigFile) Save() error {
	// Encode sensitive data into a new/temp struct
	tmpAuthConfigs := make(map[string]AuthConfig, len(configFile.AuthConfigs))
	for k, authConfig := range configFile.AuthConfigs {
		authCopy := authConfig

		authCopy.Auth = EncodeAuth(&authCopy)
		authCopy.Username = ""
		authCopy.Password = ""
		authCopy.ServerAddress = ""
		tmpAuthConfigs[k] = authCopy
	}

	saveAuthConfigs := configFile.AuthConfigs
	configFile.AuthConfigs = tmpAuthConfigs
	defer func() { configFile.AuthConfigs = saveAuthConfigs }()

	data, err := json.MarshalIndent(configFile, "", "\t")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configFile.filename), 0700); err != nil {
		return err
	}

	if err := ioutil.WriteFile(configFile.filename, data, 0600); err != nil {
		return err
	}

	return nil
}

func (config *ConfigFile) Filename() string {
	return config.filename
}

// create a base64 encoded auth string to store in config
func EncodeAuth(authConfig *AuthConfig) string {
	authStr := authConfig.Username + ":" + authConfig.Password
	msg := []byte(authStr)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(msg)))
	base64.StdEncoding.Encode(encoded, msg)
	return string(encoded)
}

// decode the auth string
func DecodeAuth(authStr string) (string, string, error) {
	decLen := base64.StdEncoding.DecodedLen(len(authStr))
	decoded := make([]byte, decLen)
	authByte := []byte(authStr)
	n, err := base64.StdEncoding.Decode(decoded, authByte)
	if err != nil {
		return "", "", err
	}
	if n > decLen {
		return "", "", fmt.Errorf("Something went wrong decoding auth config")
	}
	arr := strings.SplitN(string(decoded), ":", 2)
	if len(arr) != 2 {
		return "", "", fmt.Errorf("Invalid auth configuration file")
	}
	password := strings.Trim(arr[1], "\x00")
	return arr[0], password, nil
}
