package flags

import (
	"github.com/spf13/pflag"
)

// ClientFlags represents flags for the docker client.
type ClientFlags struct {
	FlagSet   *pflag.FlagSet
	Common    *CommonOptions
	PostParse func()

	ConfigDir string
}
