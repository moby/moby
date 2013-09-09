package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
)

// Where we store the config file
const CONFIGFILE = ".dockercfg"

// Only used for user auth + account creation
const INDEXSERVER = "https://index.docker.io/v1/"

//const INDEXSERVER = "https://indexstaging-docker.dotcloud.com/v1/"

var (
	ErrConfigFileMissing = errors.New("The Auth config file is missing")
)

type AuthConfig struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Auth          string `json:"auth"`
	Email         string `json:"email"`
	ServerAddress string `json:"serveraddress,omitempty"`
}

type ConfigFile struct {
	Configs  map[string]AuthConfig `json:"configs,omitempty"`
	rootPath string
}

func IndexServerAddress() string {
	return INDEXSERVER
}

// create a base64 encoded auth string to store in config
func encodeAuth(authConfig *AuthConfig) string {
	authStr := authConfig.Username + ":" + authConfig.Password
	msg := []byte(authStr)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(msg)))
	base64.StdEncoding.Encode(encoded, msg)
	return string(encoded)
}

// decode the auth string
func decodeAuth(authStr string) (string, string, error) {
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
	arr := strings.Split(string(decoded), ":")
	if len(arr) != 2 {
		return "", "", fmt.Errorf("Invalid auth configuration file")
	}
	password := strings.Trim(arr[1], "\x00")
	return arr[0], password, nil
}

// load up the auth config information and return values
// FIXME: use the internal golang config parser
func LoadConfig(rootPath string) (*ConfigFile, error) {
	configFile := ConfigFile{Configs: make(map[string]AuthConfig), rootPath: rootPath}
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
		authConfig := AuthConfig{}
		origAuth := strings.Split(arr[0], " = ")
		authConfig.Username, authConfig.Password, err = decodeAuth(origAuth[1])
		if err != nil {
			return &configFile, err
		}
		origEmail := strings.Split(arr[1], " = ")
		authConfig.Email = origEmail[1]
		authConfig.ServerAddress = IndexServerAddress()
		configFile.Configs[IndexServerAddress()] = authConfig
	} else {
		for k, authConfig := range configFile.Configs {
			authConfig.Username, authConfig.Password, err = decodeAuth(authConfig.Auth)
			if err != nil {
				return &configFile, err
			}
			authConfig.Auth = ""
			configFile.Configs[k] = authConfig
			authConfig.ServerAddress = k
		}
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

	configs := make(map[string]AuthConfig, len(configFile.Configs))
	for k, authConfig := range configFile.Configs {
		authCopy := authConfig

		authCopy.Auth = encodeAuth(&authCopy)
		authCopy.Username = ""
		authCopy.Password = ""
		authCopy.ServerAddress = ""
		configs[k] = authCopy
	}

	b, err := json.Marshal(configs)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(confFile, b, 0600)
	if err != nil {
		return err
	}
	return nil
}

// try to register/login to the registry server
func Login(authConfig *AuthConfig, factory *utils.HTTPRequestFactory) (string, error) {
	client := &http.Client{}
	reqStatusCode := 0
	var status string
	var reqBody []byte

	serverAddress := authConfig.ServerAddress
	if serverAddress == "" {
		serverAddress = IndexServerAddress()
	}

	loginAgainstOfficialIndex := serverAddress == IndexServerAddress()

	// to avoid sending the server address to the server it should be removed before marshalled
	authCopy := *authConfig
	authCopy.ServerAddress = ""

	jsonBody, err := json.Marshal(authCopy)
	if err != nil {
		return "", fmt.Errorf("Config Error: %s", err)
	}

	// using `bytes.NewReader(jsonBody)` here causes the server to respond with a 411 status.
	b := strings.NewReader(string(jsonBody))
	req1, err := http.Post(serverAddress+"users/", "application/json; charset=utf-8", b)
	if err != nil {
		return "", fmt.Errorf("Server Error: %s", err)
	}
	reqStatusCode = req1.StatusCode
	defer req1.Body.Close()
	reqBody, err = ioutil.ReadAll(req1.Body)
	if err != nil {
		return "", fmt.Errorf("Server Error: [%#v] %s", reqStatusCode, err)
	}

	if reqStatusCode == 201 {
		if loginAgainstOfficialIndex {
			status = "Account created. Please use the confirmation link we sent" +
				" to your e-mail to activate it."
		} else {
			status = "Account created. Please see the documentation of the registry " + serverAddress + " for instructions how to activate it."
		}
	} else if reqStatusCode == 403 {
		if loginAgainstOfficialIndex {
			return "", fmt.Errorf("Login: Your account hasn't been activated. " +
				"Please check your e-mail for a confirmation link.")
		} else {
			return "", fmt.Errorf("Login: Your account hasn't been activated. " +
				"Please see the documentation of the registry " + serverAddress + " for instructions how to activate it.")
		}
	} else if reqStatusCode == 400 {
		if string(reqBody) == "\"Username or email already exists\"" {
			req, err := factory.NewRequest("GET", serverAddress+"users/", nil)
			req.SetBasicAuth(authConfig.Username, authConfig.Password)
			resp, err := client.Do(req)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return "", err
			}
			if resp.StatusCode == 200 {
				status = "Login Succeeded"
			} else if resp.StatusCode == 401 {
				return "", fmt.Errorf("Wrong login/password, please try again")
			} else {
				return "", fmt.Errorf("Login: %s (Code: %d; Headers: %s)", body,
					resp.StatusCode, resp.Header)
			}
		} else {
			return "", fmt.Errorf("Registration: %s", reqBody)
		}
	} else {
		return "", fmt.Errorf("Unexpected status code [%d] : %s", reqStatusCode, reqBody)
	}
	return status, nil
}

// this method matches a auth configuration to a server address or a url
func (config *ConfigFile) ResolveAuthConfig(registry string) AuthConfig {
	if registry == IndexServerAddress() || len(registry) == 0 {
		// default to the index server
		return config.Configs[IndexServerAddress()]
	}
	// if its not the index server there are three cases:
	//
	// 1. this is a full config url -> it should be used as is
	// 2. it could be a full url, but with the wrong protocol
	// 3. it can be the hostname optionally with a port
	//
	// as there is only one auth entry which is fully qualified we need to start
	// parsing and matching

	swapProtocoll := func(url string) string {
		if strings.HasPrefix(url, "http:") {
			return strings.Replace(url, "http:", "https:", 1)
		}
		if strings.HasPrefix(url, "https:") {
			return strings.Replace(url, "https:", "http:", 1)
		}
		return url
	}

	resolveIgnoringProtocol := func(url string) AuthConfig {
		if c, found := config.Configs[url]; found {
			return c
		}
		registrySwappedProtocoll := swapProtocoll(url)
		// now try to match with the different protocol
		if c, found := config.Configs[registrySwappedProtocoll]; found {
			return c
		}
		return AuthConfig{}
	}

	// match both protocols as it could also be a server name like httpfoo
	if strings.HasPrefix(registry, "http:") || strings.HasPrefix(registry, "https:") {
		return resolveIgnoringProtocol(registry)
	}

	url := "https://" + registry
	if !strings.Contains(registry, "/") {
		url = url + "/v1/"
	}
	return resolveIgnoringProtocol(url)
}
