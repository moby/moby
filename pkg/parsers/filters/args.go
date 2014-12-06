package filters

import "regexp"

type Args []Arg

// Get returns the first Arg with a matching key
func (args Args) Get(key string) Arg {
	for _, arg := range args {
		if arg.Key == key {
			return arg
		}
	}
	return Arg{}
}

// GetAll returns all the Arg with a matching key
func (args Args) GetAll(key string) []Arg {
	a := []Arg{}
	for _, arg := range args {
		if arg.Key == key {
			a = append(a, arg)
		}
	}
	return a
}

type Arg struct {
	Key      string
	Operator string
	Value    string
}

func (args Args) Match(key, source string) bool {
	keyArgs := args.GetAll(key)

	//do not filter if there is no filter set or cannot determine filter
	if len(keyArgs) == 0 {
		return true
	}
	for _, arg := range keyArgs {
		match, err := regexp.MatchString(arg.Key, source)
		if err != nil {
			continue
		}
		if match {
			return true
		}
	}
	return false
}
