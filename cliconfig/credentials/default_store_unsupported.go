// +build !darwin

package credentials

import "github.com/docker/docker/cliconfig"

// DetectDefaultStore sets the default credentials store
// if the host includes the default store helper program.
// This operation is only supported in Darwin.
func DetectDefaultStore(c *cliconfig.ConfigFile) {
}
