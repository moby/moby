package configfile

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
)

const (
	// This constant is only used for really old config files when the
	// URL wasn't saved as part of the config file and it was just
	// assumed to be this value.
	defaultIndexserver = "https://index.docker.io/v1/"
)

// ConfigFile ~/.docker/config.json file info
type ConfigFile struct {
	AuthConfigs          map[string]types.AuthConfig `json:"auths"`
	HTTPHeaders          map[string]string           `json:"HttpHeaders,omitempty"`
	PsFormat             string                      `json:"psFormat,omitempty"`
	ImagesFormat         string                      `json:"imagesFormat,omitempty"`
	NetworksFormat       string                      `json:"networksFormat,omitempty"`
	PluginsFormat        string                      `json:"pluginsFormat,omitempty"`
	VolumesFormat        string                      `json:"volumesFormat,omitempty"`
	StatsFormat          string                      `json:"statsFormat,omitempty"`
	DetachKeys           string                      `json:"detachKeys,omitempty"`
	CredentialsStore     string                      `json:"credsStore,omitempty"`
	CredentialHelpers    map[string]string           `json:"credHelpers,omitempty"`
	Filename             string                      `json:"-"` // Note: for internal use only
	ServiceInspectFormat string                      `json:"serviceInspectFormat,omitempty"`
	ServicesFormat       string                      `json:"servicesFormat,omitempty"`
	TasksFormat          string                      `json:"tasksFormat,omitempty"`
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
		len(configFile.CredentialHelpers) > 0 ||
		len(configFile.AuthConfigs) > 0
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
	if configFile.Filename == "" {
		return fmt.Errorf("Can't save config with empty filename")
	}

	if err := os.MkdirAll(filepath.Dir(configFile.Filename), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(configFile.Filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return configFile.SaveToWriter(f)
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
