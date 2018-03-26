package fips

import "os"

// EnvVar is the environment variable which stores FIPS mode state
const EnvVar = "GOFIPS"

// Enabled returns true when FIPS mode is enabled
func Enabled() bool {
	return os.Getenv(EnvVar) != ""
}
