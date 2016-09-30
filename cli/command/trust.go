package command

import (
	"os"
	"strconv"

	"github.com/spf13/pflag"
)

var (
	// TODO: make this not global
	untrusted bool
)

// AddTrustedFlags adds content trust flags to the current command flagset
func AddTrustedFlags(fs *pflag.FlagSet, verify bool) {
	trusted, message := setupTrustedFlag(verify)
	fs.BoolVar(&untrusted, "disable-content-trust", !trusted, message)
}

func setupTrustedFlag(verify bool) (bool, string) {
	var trusted bool
	if e := os.Getenv("DOCKER_CONTENT_TRUST"); e != "" {
		if t, err := strconv.ParseBool(e); t || err != nil {
			// treat any other value as true
			trusted = true
		}
	}
	message := "Skip image signing"
	if verify {
		message = "Skip image verification"
	}
	return trusted, message
}

// IsTrusted returns true if content trust is enabled
func IsTrusted() bool {
	return !untrusted
}
