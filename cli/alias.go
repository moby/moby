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
	// ResolveAlias try to match name with an alias
	// if there is a match, it returns an Alias object
	ResolveAlias(name string) (alias Alias, aliasResolved bool)
}

type ComplexAlias struct {
	SimpleAlias
	CmdExecutor func(args []string) error
}

type SimpleAlias struct {
	Name string
	Cmd  []string
}

func (a SimpleAlias) GetName() string {
	return a.Name
}

func (a SimpleAlias) GetCmd() []string {
	return a.Cmd
}
