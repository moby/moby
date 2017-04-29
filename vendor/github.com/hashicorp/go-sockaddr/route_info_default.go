// +build android nacl plan9

package sockaddr

import "errors"

// getDefaultIfName is the default interface function for unsupported platforms.
func getDefaultIfName() (string, error) {
	return "", errors.New("No default interface found (unsupported platform)")
}
