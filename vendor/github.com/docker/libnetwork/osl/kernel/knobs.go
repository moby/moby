package kernel

type conditionalCheck func(val1, val2 string) bool

// OSValue represents a tuple, value defired, check function when to apply the value
type OSValue struct {
	Value   string
	CheckFn conditionalCheck
}

func propertyIsValid(val1, val2 string, check conditionalCheck) bool {
	if check == nil || check(val1, val2) {
		return true
	}
	return false
}
