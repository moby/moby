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
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth"`
	Email    string `json:"email"`
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
		configFile.Configs[IndexServerAddress()] = authConfig
	} else {
		for k, authConfig := range configFile.Configs {
			authConfig.Username, authConfig.Password, err = decodeAuth(authConfig.Auth)
			if err != nil {
				return &configFile, err
			}
			authConfig.Auth = ""
			configFile.Configs[k] = authConfig
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
	jsonBody, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("Config Error: %s", err)
	}

	// using `bytes.NewReader(jsonBody)` here causes the server to respond with a 411 status.
	b := strings.NewReader(string(jsonBody))
	req1, err := http.Post(IndexServerAddress()+"users/", "application/json; charset=utf-8", b)
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
		status = "Account created. Please use the confirmation link we sent" +
			" to your e-mail to activate it."
	} else if reqStatusCode == 403 {
		return "", fmt.Errorf("Login: Your account hasn't been activated. " +
			"Please check your e-mail for a confirmation link.")
	} else if reqStatusCode == 400 {
		if string(reqBody) == "\"Username or email already exists\"" {
			req, err := factory.NewRequest("GET", IndexServerAddress()+"users/", nil)
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
