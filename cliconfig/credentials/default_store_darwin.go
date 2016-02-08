package credentials

import (
	"os/exec"

	"github.com/docker/docker/cliconfig"
)

const defaultCredentialsStore = "osxkeychain"

// DetectDefaultStore sets the default credentials store
// if the host includes the default store helper program.
func DetectDefaultStore(c *cliconfig.ConfigFile) {
	if c.CredentialsStore != "" {
		// user defined
		return
	}

	if _, err := exec.LookPath(remoteCredentialsPrefix + c.CredentialsStore); err == nil {
		c.CredentialsStore = defaultCredentialsStore
	}
}
