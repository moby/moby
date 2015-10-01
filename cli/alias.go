package cli

// Alias represents a cli alias. Aliases can be defined in config file and through
// the alias command.
// An alias has a name and a command associated.
type Alias interface {
	GetName() string
	GetCmd() []string
}

// AliasResolver can deal with aliases
type AliasResolver interface {
	// ResolveAlias tries to match name with an alias
	// if there is a match, it returns an Alias object
	ResolveAlias(name string) (alias Alias, aliasResolved bool)
}

// ComplexAlias defines a complex alias (command starts with '!')
// this alias' command should be executed with the CmdExecutor
type ComplexAlias struct {
	SimpleAlias
	CmdExecutor func(args []string) error
}

// SimpleAlias defines an Alias with a name and a Cmd (command)
// Cmd is a valid docker command
type SimpleAlias struct {
	Name string
	Cmd  []string
}

// GetName returns the name of a
func (a SimpleAlias) GetName() string {
	return a.Name
}

// GetCmd returns the command of a
func (a SimpleAlias) GetCmd() []string {
	return a.Cmd
}
