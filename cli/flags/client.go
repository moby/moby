package flags

import flag "github.com/docker/docker/pkg/mflag"

// ClientFlags represents flags for the docker client.
type ClientFlags struct {
	FlagSet   *flag.FlagSet
	Common    *CommonFlags
	PostParse func()

	ConfigDir string
}
