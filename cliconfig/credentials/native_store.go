package credentials

import (
	"github.com/docker/docker-credential-helpers/client"
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cliconfig/configfile"
)

const (
	remoteCredentialsPrefix = "docker-credential-"
	tokenUsername           = "<token>"
)

// nativeStore implements a credentials store
// using native keychain to keep credentials secure.
// It piggybacks into a file store to keep users' emails.
type nativeStore struct {
	programFunc client.ProgramFunc
	fileStore   Store
}

// NewNativeStore creates a new native store that
// uses a remote helper program to manage credentials.
func NewNativeStore(file *configfile.ConfigFile) Store {
	name := remoteCredentialsPrefix + file.CredentialsStore
	return &nativeStore{
		programFunc: client.NewShellProgramFunc(name),
		fileStore:   NewFileStore(file),
	}
}

// Erase removes the given credentials from the native store.
func (c *nativeStore) Erase(serverAddress string) error {
	if err := client.Erase(c.programFunc, serverAddress); err != nil {
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
	creds := &credentials.Credentials{
		ServerURL: config.ServerAddress,
		Username:  config.Username,
		Secret:    config.Password,
	}

	if config.IdentityToken != "" {
		creds.Username = tokenUsername
		creds.Secret = config.IdentityToken
	}

	return client.Store(c.programFunc, creds)
}

// getCredentialsFromStore executes the command to get the credentials from the native store.
func (c *nativeStore) getCredentialsFromStore(serverAddress string) (types.AuthConfig, error) {
	var ret types.AuthConfig

	creds, err := client.Get(c.programFunc, serverAddress)
	if err != nil {
		if credentials.IsErrCredentialsNotFound(err) {
			// do not return an error if the credentials are not
			// in the keyckain. Let docker ask for new credentials.
			return ret, nil
		}
		return ret, err
	}

	if creds.Username == tokenUsername {
		ret.IdentityToken = creds.Secret
	} else {
		ret.Password = creds.Secret
		ret.Username = creds.Username
	}

	ret.ServerAddress = serverAddress
	return ret, nil
}
