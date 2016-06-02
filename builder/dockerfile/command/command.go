// Package command contains the set of Dockerfile commands.
package command

// Define constants for the command strings
const (
	Env         = "env"
	Label       = "label"
	Maintainer  = "maintainer"
	Add         = "add"
	Copy        = "copy"
	From        = "from"
	Onbuild     = "onbuild"
	Workdir     = "workdir"
	Run         = "run"
	Cmd         = "cmd"
	Entrypoint  = "entrypoint"
	Expose      = "expose"
	Volume      = "volume"
	User        = "user"
	StopSignal  = "stopsignal"
	Arg         = "arg"
	Healthcheck = "healthcheck"
)

// Commands is list of all Dockerfile commands
var Commands = map[string]struct{}{
	Env:         {},
	Label:       {},
	Maintainer:  {},
	Add:         {},
	Copy:        {},
	From:        {},
	Onbuild:     {},
	Workdir:     {},
	Run:         {},
	Cmd:         {},
	Entrypoint:  {},
	Expose:      {},
	Volume:      {},
	User:        {},
	StopSignal:  {},
	Arg:         {},
	Healthcheck: {},
}
