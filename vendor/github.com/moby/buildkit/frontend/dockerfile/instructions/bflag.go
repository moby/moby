package instructions

import (
	"fmt"
	"strings"

	"github.com/moby/buildkit/util/suggest"
)

// FlagType is the type of the build flag
type FlagType int

const (
	boolType FlagType = iota
	stringType
	stringsType
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
	bf           *BFlags
	name         string
	flagType     FlagType
	Value        string
	StringValues []string
}

// NewBFlags returns the new BFlags struct
func NewBFlags() *BFlags {
	return &BFlags{
		flags: make(map[string]*Flag),
		used:  make(map[string]*Flag),
	}
}

// NewBFlagsWithArgs returns the new BFlags struct with Args set to args
func NewBFlagsWithArgs(args []string) *BFlags {
	flags := NewBFlags()
	flags.Args = args
	return flags
}

// AddBool adds a bool flag to BFlags
// Note, any error will be generated when Parse() is called (see Parse).
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
// Note, any error will be generated when Parse() is called (see Parse).
func (bf *BFlags) AddString(name string, def string) *Flag {
	flag := bf.addFlag(name, stringType)
	if flag == nil {
		return nil
	}
	flag.Value = def
	return flag
}

// AddStrings adds a string flag to BFlags that can match multiple values
func (bf *BFlags) AddStrings(name string) *Flag {
	flag := bf.addFlag(name, stringsType)
	if flag == nil {
		return nil
	}
	return flag
}

// addFlag is a generic func used by the other AddXXX() func
// to add a new flag to the BFlags struct.
// Note, any error will be generated when Parse() is called (see Parse).
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

// Used returns a slice of flag names that are set
func (bf *BFlags) Used() []string {
	used := make([]string, 0, len(bf.used))
	for f := range bf.used {
		used = append(used, f)
	}
	return used
}

// IsTrue checks if a bool flag is true
func (fl *Flag) IsTrue() bool {
	if fl.flagType != boolType {
		// Should never get here
		panic(fmt.Errorf("Trying to use IsTrue on a non-boolean: %s", fl.name))
	}
	return fl.Value == "true"
}

// Parse parses and checks if the BFlags is valid.
// Any error noticed during the AddXXX() funcs will be generated/returned
// here.  We do this because an error during AddXXX() is more like a
// compile time error so it doesn't matter too much when we stop our
// processing as long as we do stop it, so this allows the code
// around AddXXX() to be just:
//     defFlag := AddString("description", "")
// w/o needing to add an if-statement around each one.
func (bf *BFlags) Parse() error {
	// If there was an error while defining the possible flags
	// go ahead and bubble it back up here since we didn't do it
	// earlier in the processing
	if bf.Err != nil {
		return fmt.Errorf("error setting up flags: %s", bf.Err)
	}

	for _, arg := range bf.Args {
		if !strings.HasPrefix(arg, "--") {
			return fmt.Errorf("arg should start with -- : %s", arg)
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
			return suggest.WrapError(fmt.Errorf("unknown flag: %s", arg), arg, allFlags(bf.flags), true)
		}

		if _, ok = bf.used[arg]; ok && flag.flagType != stringsType {
			return fmt.Errorf("duplicate flag specified: %s", arg)
		}

		bf.used[arg] = flag

		switch flag.flagType {
		case boolType:
			// value == "" is only ok if no "=" was specified
			if index >= 0 && value == "" {
				return fmt.Errorf("missing a value on flag: %s", arg)
			}

			lower := strings.ToLower(value)
			if lower == "" {
				flag.Value = "true"
			} else if lower == "true" || lower == "false" {
				flag.Value = lower
			} else {
				return fmt.Errorf("expecting boolean value for flag %s, not: %s", arg, value)
			}

		case stringType:
			if index < 0 {
				return fmt.Errorf("missing a value on flag: %s", arg)
			}
			flag.Value = value

		case stringsType:
			if index < 0 {
				return fmt.Errorf("missing a value on flag: %s", arg)
			}
			flag.StringValues = append(flag.StringValues, value)

		default:
			panic("No idea what kind of flag we have! Should never get here!")
		}

	}

	return nil
}

func allFlags(flags map[string]*Flag) []string {
	var names []string
	for name := range flags {
		names = append(names, name)
	}
	return names
}
