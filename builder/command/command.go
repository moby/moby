// This package contains the set of Dockerfile commands.
package command

const (
	Env        = "env"
	Maintainer = "maintainer"
	Add        = "add"
	Copy       = "copy"
	From       = "from"
	Onbuild    = "onbuild"
	Workdir    = "workdir"
	Run        = "run"
	Cmd        = "cmd"
	Entrypoint = "entrypoint"
	Expose     = "expose"
	Volume     = "volume"
	User       = "user"
	Insert     = "insert"
)

// Commands is list of all Dockerfile commands
var Commands = map[string]struct{}{
	Env:        {},
	Maintainer: {},
	Add:        {},
	Copy:       {},
	From:       {},
	Onbuild:    {},
	Workdir:    {},
	Run:        {},
	Cmd:        {},
	Entrypoint: {},
	Expose:     {},
	Volume:     {},
	User:       {},
	Insert:     {},
}
