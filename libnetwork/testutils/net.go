package testutils

import (
	"os"
)

// IsRunningInContainer returns whether the test is running inside a container.
func IsRunningInContainer() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}
