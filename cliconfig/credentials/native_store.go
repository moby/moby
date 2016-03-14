package credentials

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/engine-api/types"
)

const (
	remoteCredentialsPrefix = "docker-credential-"
	tokenUsername           = "<token>"
)

// Standarize the not found error, so every helper returns
// the same message and docker can handle it properly.
var errCredentialsNotFound = errors.New("credentials not found in native keychain")

// command is an interface that remote executed commands implement.
type command interface {
	Output() ([]byte, error)
	Input(in io.Reader)
}

// credentialsRequest holds information shared between docker and a remote credential store.
type credentialsRequest struct {
	ServerURL string
	Username  string
	Secret    string
}

// credentialsGetResponse is the information serialized from a remote store
// when the plugin sends requests to get the user credentials.
type credentialsGetResponse struct {
	Username string
	Secret   string
}

// nativeStore implements a credentials store
// using native keychain to keep credentials secure.
// It piggybacks into a file store to keep users' emails.
type nativeStore struct {
	commandFn func(args ...string) command
	fileStore Store
}

// NewNativeStore creates a new native store that
// uses a remote helper program to manage credentials.
func NewNativeStore(file *cliconfig.ConfigFile) Store {
	return &nativeStore{
		commandFn: shellCommandFn(file.CredentialsStore),
		fileStore: NewFileStore(file),
	}
}

// Erase removes the given credentials from the native store.
func (c *nativeStore) Erase(serverAddress string) error {
	if err := c.eraseCredentialsFromStore(serverAddress); err != nil {
		return err
	}

	// Fallback to plain text store to remove email
	return c.fileStore.Erase(serverAddress)
}

// Get retrieves credentials for a specific server from the native store.
func (c *nativeStore) Get(serverAddress string) (types.AuthConfig, error) {
	// load user email if it exist or an empty auth config.
	auth, _ := c.fileStore.Get(serverAddress)

	creds, err := c.getCredentialsFromStore(serverAddress)
	if err != nil {
		return auth, err
	}
	auth.Username = creds.Username
	auth.IdentityToken = creds.IdentityToken
	auth.Password = creds.Password

	return auth, nil
}

// GetAll retrieves all the credentials from the native store.
func (c *nativeStore) GetAll() (map[string]types.AuthConfig, error) {
	auths, _ := c.fileStore.GetAll()

	for s, ac := range auths {
		creds, _ := c.getCredentialsFromStore(s)
		ac.Username = creds.Username
		ac.Password = creds.Password
		ac.IdentityToken = creds.IdentityToken
		auths[s] = ac
	}

	return auths, nil
}

// Store saves the given credentials in the file store.
func (c *nativeStore) Store(authConfig types.AuthConfig) error {
	if err := c.storeCredentialsInStore(authConfig); err != nil {
		return err
	}
	authConfig.Username = ""
	authConfig.Password = ""
	authConfig.IdentityToken = ""

	// Fallback to old credential in plain text to save only the email
	return c.fileStore.Store(authConfig)
}

// storeCredentialsInStore executes the command to store the credentials in the native store.
func (c *nativeStore) storeCredentialsInStore(config types.AuthConfig) error {
	cmd := c.commandFn("store")
	creds := &credentialsRequest{
		ServerURL: config.ServerAddress,
		Username:  config.Username,
		Secret:    config.Password,
	}

	if config.IdentityToken != "" {
		creds.Username = tokenUsername
		creds.Secret = config.IdentityToken
	}

	buffer := new(bytes.Buffer)
	if err := json.NewEncoder(buffer).Encode(creds); err != nil {
		return err
	}
	cmd.Input(buffer)

	out, err := cmd.Output()
	if err != nil {
		t := strings.TrimSpace(string(out))
		logrus.Debugf("error adding credentials - err: %v, out: `%s`", err, t)
		return fmt.Errorf(t)
	}

	return nil
}

// getCredentialsFromStore executes the command to get the credentials from the native store.
func (c *nativeStore) getCredentialsFromStore(serverAddress string) (types.AuthConfig, error) {
	var ret types.AuthConfig

	cmd := c.commandFn("get")
	cmd.Input(strings.NewReader(serverAddress))

	out, err := cmd.Output()
	if err != nil {
		t := strings.TrimSpace(string(out))

		// do not return an error if the credentials are not
		// in the keyckain. Let docker ask for new credentials.
		if t == errCredentialsNotFound.Error() {
			return ret, nil
		}

		logrus.Debugf("error getting credentials - err: %v, out: `%s`", err, t)
		return ret, fmt.Errorf(t)
	}

	var resp credentialsGetResponse
	if err := json.NewDecoder(bytes.NewReader(out)).Decode(&resp); err != nil {
		return ret, err
	}

	if resp.Username == tokenUsername {
		ret.IdentityToken = resp.Secret
	} else {
		ret.Password = resp.Secret
		ret.Username = resp.Username
	}

	ret.ServerAddress = serverAddress
	return ret, nil
}

// eraseCredentialsFromStore executes the command to remove the server credentails from the native store.
func (c *nativeStore) eraseCredentialsFromStore(serverURL string) error {
	cmd := c.commandFn("erase")
	cmd.Input(strings.NewReader(serverURL))

	out, err := cmd.Output()
	if err != nil {
		t := strings.TrimSpace(string(out))
		logrus.Debugf("error erasing credentials - err: %v, out: `%s`", err, t)
		return fmt.Errorf(t)
	}

	return nil
}
