package dockerfile

// This will take a single word and an array of env variables and
// process all quotes (" and ') as well as $xxx and ${xxx} env variable
// tokens.  Tries to mimic bash shell process.
// It doesn't support all flavors of ${xx:...} formats but new ones can
// be added by adding code to the "special ${} format processing" section

import (
	"fmt"
	"strings"
	"text/scanner"
	"unicode"
)

type shellWord struct {
	word    string
	scanner scanner.Scanner
	envs    []string
	pos     int
}

// ProcessWord will use the 'env' list of environment variables,
// and replace any env var references in 'word'.
func ProcessWord(word string, env []string) (string, error) {
	sw := &shellWord{
		word: word,
		envs: env,
		pos:  0,
	}
	sw.scanner.Init(strings.NewReader(word))
	return sw.process()
}

func (sw *shellWord) process() (string, error) {
	return sw.processStopOn(scanner.EOF)
}

// Process the word, starting at 'pos', and stop when we get to the
// end of the word or the 'stopChar' character
func (sw *shellWord) processStopOn(stopChar rune) (string, error) {
	var result string
	var charFuncMapping = map[rune]func() (string, error){
		'\'': sw.processSingleQuote,
		'"':  sw.processDoubleQuote,
		'$':  sw.processDollar,
	}

	for sw.scanner.Peek() != scanner.EOF {
		ch := sw.scanner.Peek()

		if stopChar != scanner.EOF && ch == stopChar {
			sw.scanner.Next()
			break
		}
		if fn, ok := charFuncMapping[ch]; ok {
			// Call special processing func for certain chars
			tmp, err := fn()
			if err != nil {
				return "", err
			}
			result += tmp
		} else {
			// Not special, just add it to the result
			ch = sw.scanner.Next()

			if ch == '\\' {
				// '\' escapes, except end of line

				ch = sw.scanner.Next()

				if ch == scanner.EOF {
					break
				}

			}

			result += string(ch)
		}
	}

	return result, nil
}

func (sw *shellWord) processSingleQuote() (string, error) {
	// All chars between single quotes are taken as-is
	// Note, you can't escape '
	var result string

	sw.scanner.Next()

	for {
		ch := sw.scanner.Next()
		if ch == '\'' || ch == scanner.EOF {
			break
		}
		result += string(ch)
	}

	return result, nil
}

func (sw *shellWord) processDoubleQuote() (string, error) {
	// All chars up to the next " are taken as-is, even ', except any $ chars
	// But you can escape " with a \
	var result string

	sw.scanner.Next()

	for sw.scanner.Peek() != scanner.EOF {
		ch := sw.scanner.Peek()
		if ch == '"' {
			sw.scanner.Next()
			break
		}
		if ch == '$' {
			tmp, err := sw.processDollar()
			if err != nil {
				return "", err
			}
			result += tmp
		} else {
			ch = sw.scanner.Next()
			if ch == '\\' {
				chNext := sw.scanner.Peek()

				if chNext == scanner.EOF {
					// Ignore \ at end of word
					continue
				}

				if chNext == '"' || chNext == '$' {
					// \" and \$ can be escaped, all other \'s are left as-is
					ch = sw.scanner.Next()
				}
			}
			result += string(ch)
		}
	}

	return result, nil
}

func (sw *shellWord) processDollar() (string, error) {
	sw.scanner.Next()
	ch := sw.scanner.Peek()
	if ch == '{' {
		sw.scanner.Next()
		name := sw.processName()
		ch = sw.scanner.Peek()
		if ch == '}' {
			// Normal ${xx} case
			sw.scanner.Next()
			return sw.getEnv(name), nil
		}
		if ch == ':' {
			// Special ${xx:...} format processing
			// Yes it allows for recursive $'s in the ... spot

			sw.scanner.Next() // skip over :
			modifier := sw.scanner.Next()

			word, err := sw.processStopOn('}')
			if err != nil {
				return "", err
			}

			// Grab the current value of the variable in question so we
			// can use to to determine what to do based on the modifier
			newValue := sw.getEnv(name)

			switch modifier {
			case '+':
				if newValue != "" {
					newValue = word
				}
				return newValue, nil

			case '-':
				if newValue == "" {
					newValue = word
				}
				return newValue, nil

			default:
				return "", fmt.Errorf("Unsupported modifier (%c) in substitution: %s", modifier, sw.word)
			}
		}
		return "", fmt.Errorf("Missing ':' in substitution: %s", sw.word)
	}
	// $xxx case
	name := sw.processName()
	if name == "" {
		return "$", nil
	}
	return sw.getEnv(name), nil
}

func (sw *shellWord) processName() string {
	// Read in a name (alphanumeric or _)
	// If it starts with a numeric then just return $#
	var name string

	for sw.scanner.Peek() != scanner.EOF {
		ch := sw.scanner.Peek()
		if len(name) == 0 && unicode.IsDigit(ch) {
			ch = sw.scanner.Next()
			return string(ch)
		}
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			break
		}
		ch = sw.scanner.Next()
		name += string(ch)
	}

	return name
}

func (sw *shellWord) getEnv(name string) string {
	for _, env := range sw.envs {
		i := strings.Index(env, "=")
		if i < 0 {
			if name == env {
				// Should probably never get here, but just in case treat
				// it like "var" and "var=" are the same
				return ""
			}
			continue
		}
		if name != env[:i] {
			continue
		}
		return env[i+1:]
	}
	return ""
}
