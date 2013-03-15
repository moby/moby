package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// Where we store the config file
const CONFIGFILE = "/var/lib/docker/.dockercfg"

// the registry server we want to login against
const REGISTRY_SERVER = "http://registry.docker.io"

type AuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

// create a base64 encoded auth string to store in config
func EncodeAuth(authConfig AuthConfig) string {
	authStr := authConfig.Username + ":" + authConfig.Password
	msg := []byte(authStr)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(msg)))
	base64.StdEncoding.Encode(encoded, msg)
	return string(encoded)
}

// decode the auth string
func DecodeAuth(authStr string) (AuthConfig, error) {
	decLen := base64.StdEncoding.DecodedLen(len(authStr))
	decoded := make([]byte, decLen)
	authByte := []byte(authStr)
	n, err := base64.StdEncoding.Decode(decoded, authByte)
	if err != nil {
		return AuthConfig{}, err
	}
	if n > decLen {
		return AuthConfig{}, errors.New("something went wrong decoding auth config")
	}
	arr := strings.Split(string(decoded), ":")
	password := strings.Trim(arr[1], "\x00")
	return AuthConfig{Username: arr[0], Password: password}, nil

}

// load up the auth config information and return values
func LoadConfig() (AuthConfig, error) {
	if _, err := os.Stat(CONFIGFILE); err == nil {
		b, err := ioutil.ReadFile(CONFIGFILE)
		if err != nil {
			return AuthConfig{}, err
		}
		arr := strings.Split(string(b), "\n")
		orig_auth := strings.Split(arr[0], " = ")
		orig_email := strings.Split(arr[1], " = ")
		authConfig, err := DecodeAuth(orig_auth[1])
		if err != nil {
			return AuthConfig{}, err
		}
		authConfig.Email = orig_email[1]
		return authConfig, nil
	} else {
		return AuthConfig{}, nil
	}
	return AuthConfig{}, nil
}

// save the auth config
func saveConfig(authStr string, email string) error {
	lines := "auth = " + authStr + "\n" + "email = " + email + "\n"
	b := []byte(lines)
	err := ioutil.WriteFile(CONFIGFILE, b, 0600)
	if err != nil {
		return err
	}
	return nil
}

// try to register/login to the registry server
func Login(authConfig AuthConfig) (string, error) {
	storeConfig := false
	reqStatusCode := 0
	var status string
	var reqBody []byte
	jsonBody, err := json.Marshal(authConfig)
	if err != nil {
		errMsg = fmt.Sprintf("Config Error: %s", err)
		return "", errors.New(errMsg)
	}

	b := strings.NewReader(string(jsonBody))
	req1, err := http.Post(REGISTRY_SERVER+"/v1/users", "application/json; charset=utf-8", b)
	if err != nil {
		errMsg = fmt.Sprintf("Server Error: %s", err)
		return "", errors.New(errMsg)
	}

	reqStatusCode = req1.StatusCode
	defer req1.Body.Close()
	reqBody, err = ioutil.ReadAll(req1.Body)
	if err != nil {
		errMsg = fmt.Sprintf("Server Error: [%s] %s", reqStatusCode, err)
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
		saveConfig(authStr, authConfig.Email)
	}
	return status, nil
}
