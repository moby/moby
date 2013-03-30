package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
)

// Where we store the config file
const CONFIGFILE = ".dockercfg"

// the registry server we want to login against
const REGISTRY_SERVER = "https://registry.docker.io"

type AuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	rootPath string `json:-`
}

func NewAuthConfig(username, password, email, rootPath string) *AuthConfig {
	return &AuthConfig{
		Username: username,
		Password: password,
		Email:    email,
		rootPath: rootPath,
	}
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
func DecodeAuth(authStr string) (*AuthConfig, error) {
	decLen := base64.StdEncoding.DecodedLen(len(authStr))
	decoded := make([]byte, decLen)
	authByte := []byte(authStr)
	n, err := base64.StdEncoding.Decode(decoded, authByte)
	if err != nil {
		return nil, err
	}
	if n > decLen {
		return nil, fmt.Errorf("Something went wrong decoding auth config")
	}
	arr := strings.Split(string(decoded), ":")
	if len(arr) != 2 {
		return nil, fmt.Errorf("Invalid auth configuration file")
	}
	password := strings.Trim(arr[1], "\x00")
	return &AuthConfig{Username: arr[0], Password: password}, nil

}

// load up the auth config information and return values
// FIXME: use the internal golang config parser
func LoadConfig(rootPath string) (*AuthConfig, error) {
	confFile := path.Join(rootPath, CONFIGFILE)
	if _, err := os.Stat(confFile); err != nil {
		return &AuthConfig{}, fmt.Errorf("The Auth config file is missing")
	}
	b, err := ioutil.ReadFile(confFile)
	if err != nil {
		return nil, err
	}
	arr := strings.Split(string(b), "\n")
	origAuth := strings.Split(arr[0], " = ")
	origEmail := strings.Split(arr[1], " = ")
	authConfig, err := DecodeAuth(origAuth[1])
	if err != nil {
		return nil, err
	}
	authConfig.Email = origEmail[1]
	authConfig.rootPath = rootPath
	return authConfig, nil
}

// save the auth config
func saveConfig(rootPath, authStr string, email string) error {
	lines := "auth = " + authStr + "\n" + "email = " + email + "\n"
	b := []byte(lines)
	err := ioutil.WriteFile(path.Join(rootPath, CONFIGFILE), b, 0600)
	if err != nil {
		return err
	}
	return nil
}

// try to register/login to the registry server
func Login(authConfig *AuthConfig) (string, error) {
	storeConfig := false
	reqStatusCode := 0
	var status string
	var errMsg string
	var reqBody []byte
	jsonBody, err := json.Marshal(authConfig)
	if err != nil {
		errMsg = fmt.Sprintf("Config Error: %s", err)
		return "", errors.New(errMsg)
	}

	b := bytes.NewReader(jsonBody)
	req1, err := http.Post(REGISTRY_SERVER+"/v1/users", "application/json; charset=utf-8", b)
	if err != nil {
		errMsg = fmt.Sprintf("Server Error: %s", err)
		return "", errors.New(errMsg)
	}

	reqStatusCode = req1.StatusCode
	defer req1.Body.Close()
	reqBody, err = ioutil.ReadAll(req1.Body)
	if err != nil {
		errMsg = fmt.Sprintf("Server Error: [%#v] %s", reqStatusCode, err)
		return "", errors.New(errMsg)
	}

	if reqStatusCode == 201 {
		status = "Account Created\n"
		storeConfig = true
	} else if reqStatusCode == 400 {
		if string(reqBody) == "Username or email already exist" {
			client := &http.Client{}
			req, err := http.NewRequest("GET", REGISTRY_SERVER+"/v1/users", nil)
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
				status = "Login Succeeded\n"
				storeConfig = true
			} else {
				status = fmt.Sprintf("Login: %s", body)
				return "", errors.New(status)
			}
		} else {
			status = fmt.Sprintf("Registration: %s", string(reqBody))
			return "", errors.New(status)
		}
	} else {
		status = fmt.Sprintf("[%s] : %s", reqStatusCode, string(reqBody))
		return "", errors.New(status)
	}
	if storeConfig {
		authStr := EncodeAuth(authConfig)
		saveConfig(authConfig.rootPath, authStr, authConfig.Email)
	}
	return status, nil
}
