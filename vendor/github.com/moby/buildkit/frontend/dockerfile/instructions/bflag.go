package instructions

import (
	"strings"

	"github.com/moby/buildkit/util/suggest"
	"github.com/pkg/errors"
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
		bf.Err = errors.Errorf("Duplicate flag defined: %s", name)
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
		err := errors.Errorf("Trying to use IsTrue on a non-boolean: %s", fl.name)
		panic(err)
	}
	return fl.Value == "true"
}

// Parse parses and checks if the BFlags is valid.
// Any error noticed during the AddXXX() funcs will be generated/returned
// here.  We do this because an error during AddXXX() is more like a
// compile time error so it doesn't matter too much when we stop our
// processing as long as we do stop it, so this allows the code
// around AddXXX() to be just:
//
//	defFlag := AddString("description", "")
//
// w/o needing to add an if-statement around each one.
func (bf *BFlags) Parse() error {
	// If there was an error while defining the possible flags
	// go ahead and bubble it back up here since we didn't do it
	// earlier in the processing
	if bf.Err != nil {
		return errors.Wrap(bf.Err, "error setting up flags")
	}

	for _, a := range bf.Args {
		if a == "--" {
			// Stop processing further arguments as flags. We're matching
			// the POSIX Utility Syntax Guidelines here;
			// https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap12.html#tag_12_02
			//
			// > The first -- argument that is not an option-argument should be accepted
			// > as a delimiter indicating the end of options. Any following arguments
			// > should be treated as operands, even if they begin with the '-' character.
			return nil
		}
		if !strings.HasPrefix(a, "--") {
			return errors.Errorf("arg should start with -- : %s", a)
		}

		flagName, value, hasValue := strings.Cut(a, "=")
		arg := flagName[2:]

		flag, ok := bf.flags[arg]
		if !ok {
			err := errors.Errorf("unknown flag: %s", flagName)
			return suggest.WrapError(err, arg, allFlags(bf.flags), true)
		}

		if _, ok = bf.used[arg]; ok && flag.flagType != stringsType {
			return errors.Errorf("duplicate flag specified: %s", flagName)
		}

		bf.used[arg] = flag

		switch flag.flagType {
		case boolType:
			// value == "" is only ok if no "=" was specified
			if hasValue && value == "" {
				return errors.Errorf("missing a value on flag: %s", flagName)
			}

			switch strings.ToLower(value) {
			case "true", "":
				flag.Value = "true"
			case "false":
				flag.Value = "false"
			default:
				return errors.Errorf("expecting boolean value for flag %s, not: %s", flagName, value)
			}

		case stringType:
			if !hasValue {
				return errors.Errorf("missing a value on flag: %s", flagName)
			}
			flag.Value = value

		case stringsType:
			if !hasValue {
				return errors.Errorf("missing a value on flag: %s", flagName)
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
