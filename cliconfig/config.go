package cliconfig

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/homedir"
	"github.com/docker/engine-api/types"
)

const (
	// ConfigFileName is the name of config file
	ConfigFileName = "config.json"
	configFileDir  = ".docker"
	oldConfigfile  = ".dockercfg"

	// This constant is only used for really old config files when the
	// URL wasn't saved as part of the config file and it was just
	// assumed to be this value.
	defaultIndexserver = "https://index.docker.io/v1/"
)

var (
	configDir = os.Getenv("DOCKER_CONFIG")
)

func init() {
	if configDir == "" {
		configDir = filepath.Join(homedir.Get(), configFileDir)
	}
}

// ConfigDir returns the directory the configuration file is stored in
func ConfigDir() string {
	return configDir
}

// SetConfigDir sets the directory the configuration file is stored in
func SetConfigDir(dir string) {
	configDir = dir
}

// ConfigFile ~/.docker/config.json file info
type ConfigFile struct {
	AuthConfigs      map[string]types.AuthConfig `json:"auths"`
	HTTPHeaders      map[string]string           `json:"HttpHeaders,omitempty"`
	PsFormat         string                      `json:"psFormat,omitempty"`
	ImagesFormat     string                      `json:"imagesFormat,omitempty"`
	DetachKeys       string                      `json:"detachKeys,omitempty"`
	CredentialsStore string                      `json:"credsStore,omitempty"`
	filename         string                      // Note: not serialized - for internal use only
}

// NewConfigFile initializes an empty configuration file for the given filename 'fn'
func NewConfigFile(fn string) *ConfigFile {
	return &ConfigFile{
		AuthConfigs: make(map[string]types.AuthConfig),
		HTTPHeaders: make(map[string]string),
		filename:    fn,
	}
}

// LegacyLoadFromReader reads the non-nested configuration data given and sets up the
// auth config information with given directory and populates the receiver object
func (configFile *ConfigFile) LegacyLoadFromReader(configData io.Reader) error {
	b, err := ioutil.ReadAll(configData)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(b, &configFile.AuthConfigs); err != nil {
		arr := strings.Split(string(b), "\n")
		if len(arr) < 2 {
			return fmt.Errorf("The Auth config file is empty")
		}
		authConfig := types.AuthConfig{}
		origAuth := strings.Split(arr[0], " = ")
		if len(origAuth) != 2 {
			return fmt.Errorf("Invalid Auth config file")
		}
		authConfig.Username, authConfig.Password, err = decodeAuth(origAuth[1])
		if err != nil {
			return err
		}
		authConfig.ServerAddress = defaultIndexserver
		configFile.AuthConfigs[defaultIndexserver] = authConfig
	} else {
		for k, authConfig := range configFile.AuthConfigs {
			authConfig.Username, authConfig.Password, err = decodeAuth(authConfig.Auth)
			if err != nil {
				return err
			}
			authConfig.Auth = ""
			authConfig.ServerAddress = k
			configFile.AuthConfigs[k] = authConfig
		}
	}
	return nil
}

// LoadFromReader reads the configuration data given and sets up the auth config
// information with given directory and populates the receiver object
func (configFile *ConfigFile) LoadFromReader(configData io.Reader) error {
	if err := json.NewDecoder(configData).Decode(&configFile); err != nil {
		return err
	}
	var err error
	for addr, ac := range configFile.AuthConfigs {
		ac.Username, ac.Password, err = decodeAuth(ac.Auth)
		if err != nil {
			return err
		}
		ac.Auth = ""
		ac.ServerAddress = addr
		configFile.AuthConfigs[addr] = ac
	}
	return nil
}

// ContainsAuth returns whether there is authentication configured
// in this file or not.
func (configFile *ConfigFile) ContainsAuth() bool {
	return configFile.CredentialsStore != "" ||
		(configFile.AuthConfigs != nil && len(configFile.AuthConfigs) > 0)
}

// LegacyLoadFromReader is a convenience function that creates a ConfigFile object from
// a non-nested reader
func LegacyLoadFromReader(configData io.Reader) (*ConfigFile, error) {
	configFile := ConfigFile{
		AuthConfigs: make(map[string]types.AuthConfig),
	}
	err := configFile.LegacyLoadFromReader(configData)
	return &configFile, err
}

// LoadFromReader is a convenience function that creates a ConfigFile object from
// a reader
func LoadFromReader(configData io.Reader) (*ConfigFile, error) {
	configFile := ConfigFile{
		AuthConfigs: make(map[string]types.AuthConfig),
	}
	err := configFile.LoadFromReader(configData)
	return &configFile, err
}

// Load reads the configuration files in the given directory, and sets up
// the auth config information and return values.
// FIXME: use the internal golang config parser
func Load(configDir string) (*ConfigFile, error) {
	if configDir == "" {
		configDir = ConfigDir()
	}

	configFile := ConfigFile{
		AuthConfigs: make(map[string]types.AuthConfig),
		filename:    filepath.Join(configDir, ConfigFileName),
	}

	// Try happy path first - latest config file
	if _, err := os.Stat(configFile.filename); err == nil {
		file, err := os.Open(configFile.filename)
		if err != nil {
			return &configFile, fmt.Errorf("%s - %v", configFile.filename, err)
		}
		defer file.Close()
		err = configFile.LoadFromReader(file)
		if err != nil {
			err = fmt.Errorf("%s - %v", configFile.filename, err)
		}
		return &configFile, err
	} else if !os.IsNotExist(err) {
		// if file is there but we can't stat it for any reason other
		// than it doesn't exist then stop
		return &configFile, fmt.Errorf("%s - %v", configFile.filename, err)
	}

	// Can't find latest config file so check for the old one
	confFile := filepath.Join(homedir.Get(), oldConfigfile)
	if _, err := os.Stat(confFile); err != nil {
		return &configFile, nil //missing file is not an error
	}
	file, err := os.Open(confFile)
	if err != nil {
		return &configFile, fmt.Errorf("%s - %v", confFile, err)
	}
	defer file.Close()
	err = configFile.LegacyLoadFromReader(file)
	if err != nil {
		return &configFile, fmt.Errorf("%s - %v", confFile, err)
	}

	if configFile.HTTPHeaders == nil {
		configFile.HTTPHeaders = map[string]string{}
	}
	return &configFile, nil
}

// SaveToWriter encodes and writes out all the authorization information to
// the given writer
func (configFile *ConfigFile) SaveToWriter(writer io.Writer) error {
	// Encode sensitive data into a new/temp struct
	tmpAuthConfigs := make(map[string]types.AuthConfig, len(configFile.AuthConfigs))
	for k, authConfig := range configFile.AuthConfigs {
		authCopy := authConfig
		// encode and save the authstring, while blanking out the original fields
		authCopy.Auth = encodeAuth(&authCopy)
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
	_, err = writer.Write(data)
	return err
}

// Save encodes and writes out all the authorization information
func (configFile *ConfigFile) Save() error {
	if configFile.Filename() == "" {
		return fmt.Errorf("Can't save config with empty filename")
	}

	if err := os.MkdirAll(filepath.Dir(configFile.filename), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(configFile.filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return configFile.SaveToWriter(f)
}

// Filename returns the name of the configuration file
func (configFile *ConfigFile) Filename() string {
	return configFile.filename
}

// encodeAuth creates a base64 encoded string to containing authorization information
func encodeAuth(authConfig *types.AuthConfig) string {
	if authConfig.Username == "" && authConfig.Password == "" {
		return ""
	}

	authStr := authConfig.Username + ":" + authConfig.Password
	msg := []byte(authStr)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(msg)))
	base64.StdEncoding.Encode(encoded, msg)
	return string(encoded)
}

// decodeAuth decodes a base64 encoded string and returns username and password
func decodeAuth(authStr string) (string, string, error) {
	if authStr == "" {
		return "", "", nil
	}

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
