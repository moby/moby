package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/config"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/dotcloud/docker/utils"
)

// Only used for user auth + account creation
const INDEXSERVER = "https://index.docker.io/v1/"

//const INDEXSERVER = "https://indexstaging-docker.dotcloud.com/v1/"

type AuthConfig struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Auth          string `json:"auth"`
	Email         string `json:"email"`
	ServerAddress string `json:"serveraddress,omitempty"`
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
	arr := strings.SplitN(string(decoded), ":", 2)
	if len(arr) != 2 {
		return "", "", fmt.Errorf("Invalid auth configuration file")
	}
	password := strings.Trim(arr[1], "\x00")
	return arr[0], password, nil
}

func GetAuth(config *config.ConfigFile, key string) (*AuthConfig, error) {
	a, err := config.GetConfig(key)
	if err != nil {
		return nil, err
	}
	authConfig, found := a.(*AuthConfig)
	if !found {
		return &AuthConfig{}, fmt.Errorf("%s value is invalid type", key)
	}
	if authConfig.Auth != "" {
		authConfig.Username, authConfig.Password, err = decodeAuth(authConfig.Auth)
		if err != nil {
			return authConfig, err
		}
	}
	authConfig.Auth = ""
	authConfig.ServerAddress = key
	return authConfig, nil
}

func PutAuth(config *config.ConfigFile, key string, authConfig *AuthConfig) error {
	authCopy := authConfig

	authCopy.Auth = encodeAuth(authCopy)
	authCopy.Username = ""
	authCopy.Password = ""
	authCopy.ServerAddress = ""

	return config.PutConfig(key, authCopy)
}

// try to register/login to the registry server
func Login(authConfig *AuthConfig, factory *utils.HTTPRequestFactory) (string, error) {
	var (
		status  string
		reqBody []byte
		err     error
		client  = &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
				Proxy:             http.ProxyFromEnvironment,
			},
			CheckRedirect: AddRequiredHeadersToRedirectedRequests,
		}
		reqStatusCode = 0
		serverAddress = authConfig.ServerAddress
	)

	if serverAddress == "" {
		serverAddress = IndexServerAddress()
	}

	loginAgainstOfficialIndex := serverAddress == IndexServerAddress()

	// to avoid sending the server address to the server it should be removed before being marshalled
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
			} else if resp.StatusCode == 403 {
				if loginAgainstOfficialIndex {
					return "", fmt.Errorf("Login: Account is not Active. Please check your e-mail for a confirmation link.")
				}
				return "", fmt.Errorf("Login: Account is not Active. Please see the documentation of the registry %s for instructions how to activate it.", serverAddress)
			} else {
				return "", fmt.Errorf("Login: %s (Code: %d; Headers: %s)", body, resp.StatusCode, resp.Header)
			}
		} else {
			return "", fmt.Errorf("Registration: %s", reqBody)
		}
	} else if reqStatusCode == 401 {
		// This case would happen with private registries where /v1/users is
		// protected, so people can use `docker login` as an auth check.
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
		return "", fmt.Errorf("Unexpected status code [%d] : %s", reqStatusCode, reqBody)
	}
	return status, nil
}

// this method matches a auth configuration to a server address or a url
func ResolveAuthConfig(config *config.ConfigFile, hostname string) AuthConfig {
	if len(hostname) == 0 {
		// default to the index server
		hostname = IndexServerAddress()
	}

	// First try the happy case
	if c, err := GetAuth(config, hostname); err == nil {
		return *c
	}

	convertToHostname := func(url string) string {
		stripped := url
		if strings.HasPrefix(url, "http://") {
			stripped = strings.Replace(url, "http://", "", 1)
		} else if strings.HasPrefix(url, "https://") {
			stripped = strings.Replace(url, "https://", "", 1)
		}

		nameParts := strings.SplitN(stripped, "/", 2)

		return nameParts[0]
	}

	normalizedHostename := convertToHostname(hostname)
	for index, _ := range config.Configs {
		if registryHostname := convertToHostname(index); registryHostname == normalizedHostename {
			if c, err := GetAuth(config, index); err == nil {
				return *c
			}
		}
	}

	// When all else fails, return an empty auth config
	return AuthConfig{}
}
