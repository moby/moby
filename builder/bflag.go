package builder

import (
	"fmt"
	"strings"
)

// FlagType is the type of the build flag
type FlagType int

const (
	boolType FlagType = iota
	stringType
)

// BFlags contains all flags information for the builder
type BFlags struct {
	Args  []string // actual flags/args from cmd line
	flags map[string]*Flag
	used  map[string]*Flag
	Err   error
}

// Flag contains all information for a flag
type Flag struct {
	bf       *BFlags
	name     string
	flagType FlagType
	Value    string
}

// NewBFlags return the new BFlags struct
func NewBFlags() *BFlags {
	return &BFlags{
		flags: make(map[string]*Flag),
		used:  make(map[string]*Flag),
	}
}

// AddBool adds a bool flag to BFlags
func (bf *BFlags) AddBool(name string, def bool) *Flag {
	flag := bf.addFlag(name, boolType)
	if flag == nil {
		return nil
	}
	if def {
		flag.Value = "true"
	} else {
		flag.Value = "false"
	}
	return flag
}

// AddString adds a string flag to BFlags
func (bf *BFlags) AddString(name string, def string) *Flag {
	flag := bf.addFlag(name, stringType)
	if flag == nil {
		return nil
	}
	flag.Value = def
	return flag
}

func (bf *BFlags) addFlag(name string, flagType FlagType) *Flag {
	if _, ok := bf.flags[name]; ok {
		bf.Err = fmt.Errorf("Duplicate flag defined: %s", name)
		return nil
	}

	newFlag := &Flag{
		bf:       bf,
		name:     name,
		flagType: flagType,
	}
	bf.flags[name] = newFlag

	return newFlag
}

// IsUsed checks if the flag is used
func (fl *Flag) IsUsed() bool {
	if _, ok := fl.bf.used[fl.name]; ok {
		return true
	}
	return false
}

// IsTrue checks if a bool flag is true
func (fl *Flag) IsTrue() bool {
	if fl.flagType != boolType {
		// Should never get here
		panic(fmt.Errorf("Trying to use IsTrue on a non-boolean: %s", fl.name))
	}
	return fl.Value == "true"
}

// Parse parses and checks if the BFlags is valid
func (bf *BFlags) Parse() error {
	// If there was an error while defining the possible flags
	// go ahead and bubble it back up here since we didn't do it
	// earlier in the processing
	if bf.Err != nil {
		return fmt.Errorf("Error setting up flags: %s", bf.Err)
	}

	for _, arg := range bf.Args {
		if !strings.HasPrefix(arg, "--") {
			return fmt.Errorf("Arg should start with -- : %s", arg)
		}

		if arg == "--" {
			return nil
		}

		arg = arg[2:]
		value := ""

		index := strings.Index(arg, "=")
		if index >= 0 {
			value = arg[index+1:]
			arg = arg[:index]
		}

		flag, ok := bf.flags[arg]
		if !ok {
			return fmt.Errorf("Unknown flag: %s", arg)
		}

		if _, ok = bf.used[arg]; ok {
			return fmt.Errorf("Duplicate flag specified: %s", arg)
		}

		bf.used[arg] = flag

		switch flag.flagType {
		case boolType:
			// value == "" is only ok if no "=" was specified
			if index >= 0 && value == "" {
				return fmt.Errorf("Missing a value on flag: %s", arg)
			}

			lower := strings.ToLower(value)
			if lower == "" {
				flag.Value = "true"
			} else if lower == "true" || lower == "false" {
				flag.Value = lower
			} else {
				return fmt.Errorf("Expecting boolean value for flag %s, not: %s", arg, value)
			}

		case stringType:
			if index < 0 {
				return fmt.Errorf("Missing a value on flag: %s", arg)
			}
			flag.Value = value

		default:
			panic(fmt.Errorf("No idea what kind of flag we have! Should never get here!"))
		}

	}

	return nil
}
