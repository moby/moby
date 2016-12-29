package opts

// QuotedString is a string that may have extra quotes around the value. The
// quotes are stripped from the value.
type QuotedString string

// Set sets a new value
func (s *QuotedString) Set(val string) error {
	*s = QuotedString(trimQuotes(val))
	return nil
}

// Type returns the type of the value
func (s *QuotedString) Type() string {
	return "string"
}

func (s *QuotedString) String() string {
	return string(*s)
}

func trimQuotes(value string) string {
	lastIndex := len(value) - 1
	for _, char := range []byte{'\'', '"'} {
		if value[0] == char && value[lastIndex] == char {
			return value[1:lastIndex]
		}
	}
	return value
}
