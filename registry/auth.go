package registry

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/utils"
)

const (
	// Where we store the config file
	CONFIGFILE = ".dockercfg"
)

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
		if len(origAuth) != 2 {
			return &configFile, fmt.Errorf("Invalid Auth config file")
		}
		authConfig.Username, authConfig.Password, err = decodeAuth(origAuth[1])
		if err != nil {
			return &configFile, err
		}
		origEmail := strings.Split(arr[1], " = ")
		if len(origEmail) != 2 {
			return &configFile, fmt.Errorf("Invalid Auth config file")
		}
		authConfig.Email = origEmail[1]
		authConfig.ServerAddress = IndexServerAddress()
		// *TODO: Switch to using IndexServerName() instead?
		configFile.Configs[IndexServerAddress()] = authConfig
	} else {
		for k, authConfig := range configFile.Configs {
			authConfig.Username, authConfig.Password, err = decodeAuth(authConfig.Auth)
			if err != nil {
				return &configFile, err
			}
			authConfig.Auth = ""
			authConfig.ServerAddress = k
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

// Login tries to register/login to the registry server.
func Login(authConfig *AuthConfig, registryEndpoint *Endpoint, factory *utils.HTTPRequestFactory) (string, error) {
	// Separates the v2 registry login logic from the v1 logic.
	if registryEndpoint.Version == APIVersion2 {
		return loginV2(authConfig, registryEndpoint, factory)
	}

	return loginV1(authConfig, registryEndpoint, factory)
}

// loginV1 tries to register/login to the v1 registry server.
func loginV1(authConfig *AuthConfig, registryEndpoint *Endpoint, factory *utils.HTTPRequestFactory) (string, error) {
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

	log.Debugf("attempting v1 login to registry endpoint %s", registryEndpoint)

	if serverAddress == "" {
		return "", fmt.Errorf("Server Error: Server Address not set.")
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
			// *TODO: Use registry configuration to determine what this says, if anything?
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
				return "Login Succeeded", nil
			} else if resp.StatusCode == 401 {
				return "", fmt.Errorf("Wrong login/password, please try again")
			} else if resp.StatusCode == 403 {
				if loginAgainstOfficialIndex {
					return "", fmt.Errorf("Login: Account is not Active. Please check your e-mail for a confirmation link.")
				}
				// *TODO: Use registry configuration to determine what this says, if anything?
				return "", fmt.Errorf("Login: Account is not Active. Please see the documentation of the registry %s for instructions how to activate it.", serverAddress)
			}
			return "", fmt.Errorf("Login: %s (Code: %d; Headers: %s)", body, resp.StatusCode, resp.Header)
		}
		return "", fmt.Errorf("Registration: %s", reqBody)

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
			return "Login Succeeded", nil
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

// loginV2 tries to login to the v2 registry server. The given registry endpoint has been
// pinged or setup with a list of authorization challenges. Each of these challenges are
// tried until one of them succeeds. Currently supported challenge schemes are:
// 		HTTP Basic Authorization
// 		Token Authorization with a separate token issuing server
// NOTE: the v2 logic does not attempt to create a user account if one doesn't exist. For
// now, users should create their account through other means like directly from a web page
// served by the v2 registry service provider. Whether this will be supported in the future
// is to be determined.
func loginV2(authConfig *AuthConfig, registryEndpoint *Endpoint, factory *utils.HTTPRequestFactory) (string, error) {
	log.Debugf("attempting v2 login to registry endpoint %s", registryEndpoint)

	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			Proxy:             http.ProxyFromEnvironment,
		},
		CheckRedirect: AddRequiredHeadersToRedirectedRequests,
	}

	var (
		err       error
		allErrors []error
	)

	for _, challenge := range registryEndpoint.AuthChallenges {
		log.Debugf("trying %q auth challenge with params %s", challenge.Scheme, challenge.Parameters)

		switch strings.ToLower(challenge.Scheme) {
		case "basic":
			err = tryV2BasicAuthLogin(authConfig, challenge.Parameters, registryEndpoint, client, factory)
		case "bearer":
			err = tryV2TokenAuthLogin(authConfig, challenge.Parameters, registryEndpoint, client, factory)
		default:
			// Unsupported challenge types are explicitly skipped.
			err = fmt.Errorf("unsupported auth scheme: %q", challenge.Scheme)
		}

		if err == nil {
			return "Login Succeeded", nil
		}

		log.Debugf("error trying auth challenge %q: %s", challenge.Scheme, err)

		allErrors = append(allErrors, err)
	}

	return "", fmt.Errorf("no successful auth challenge for %s - errors: %s", registryEndpoint, allErrors)
}

func tryV2BasicAuthLogin(authConfig *AuthConfig, params map[string]string, registryEndpoint *Endpoint, client *http.Client, factory *utils.HTTPRequestFactory) error {
	req, err := factory.NewRequest("GET", registryEndpoint.Path(""), nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(authConfig.Username, authConfig.Password)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("basic auth attempt to %s realm %q failed with status: %d %s", registryEndpoint, params["realm"], resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return nil
}

func tryV2TokenAuthLogin(authConfig *AuthConfig, params map[string]string, registryEndpoint *Endpoint, client *http.Client, factory *utils.HTTPRequestFactory) error {
	token, err := getToken(authConfig.Username, authConfig.Password, params, registryEndpoint, client, factory)
	if err != nil {
		return err
	}

	req, err := factory.NewRequest("GET", registryEndpoint.Path(""), nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token auth attempt to %s realm %q failed with status: %d %s", registryEndpoint, params["realm"], resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return nil
}

// this method matches a auth configuration to a server address or a url
func (config *ConfigFile) ResolveAuthConfig(index *IndexInfo) AuthConfig {
	configKey := index.GetAuthConfigKey()
	// First try the happy case
	if c, found := config.Configs[configKey]; found || index.Official {
		return c
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

	// Maybe they have a legacy config file, we will iterate the keys converting
	// them to the new format and testing
	for registry, config := range config.Configs {
		if configKey == convertToHostname(registry) {
			return config
		}
	}

	// When all else fails, return an empty auth config
	return AuthConfig{}
}
