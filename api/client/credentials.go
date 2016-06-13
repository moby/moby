package client

import (
	"github.com/docker/docker/cliconfig/configfile"
	"github.com/docker/docker/cliconfig/credentials"
	"github.com/docker/engine-api/types"
)

// GetCredentials loads the user credentials from a credentials store.
// The store is determined by the config file settings.
func GetCredentials(c *configfile.ConfigFile, serverAddress string) (types.AuthConfig, error) {
	s := LoadCredentialsStore(c)
	return s.Get(serverAddress)
}

// GetAllCredentials loads all credentials from a credentials store.
// The store is determined by the config file settings.
func GetAllCredentials(c *configfile.ConfigFile) (map[string]types.AuthConfig, error) {
	s := LoadCredentialsStore(c)
	return s.GetAll()
}

// StoreCredentials saves the user credentials in a credentials store.
// The store is determined by the config file settings.
func StoreCredentials(c *configfile.ConfigFile, auth types.AuthConfig) error {
	s := LoadCredentialsStore(c)
	return s.Store(auth)
}

// EraseCredentials removes the user credentials from a credentials store.
// The store is determined by the config file settings.
func EraseCredentials(c *configfile.ConfigFile, serverAddress string) error {
	s := LoadCredentialsStore(c)
	return s.Erase(serverAddress)
}

// LoadCredentialsStore initializes a new credentials store based
// in the settings provided in the configuration file.
func LoadCredentialsStore(c *configfile.ConfigFile) credentials.Store {
	if c.CredentialsStore != "" {
		return credentials.NewNativeStore(c)
	}
	return credentials.NewFileStore(c)
}
