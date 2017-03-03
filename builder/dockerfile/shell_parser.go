package dockerfile

// This will take a single word and an array of env variables and
// process all quotes (" and ') as well as $xxx and ${xxx} env variable
// tokens.  Tries to mimic bash shell process.
// It doesn't support all flavors of ${xx:...} formats but new ones can
// be added by adding code to the "special ${} format processing" section

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"text/scanner"
	"unicode"
)

type shellWord struct {
	word            string
	scanner         scanner.Scanner
	envs            []string
	lazyEnvVars     []string
	pos             int
	escapeToken     rune
	allowLazyExpand bool
	hasLazyExpand   bool
}

// ProcessWord will use the 'env' list of environment variables,
// and replace any env var references in 'word'.
// if allowLazyExpand is true, detects undeclared variables and don't replace it with empty string
func ProcessWord(word string, env, lazyEnvVars []string, escapeToken rune, allowLazyExpand bool) (outWord string, isLazyExpanded bool, err error) {
	sw := &shellWord{
		word:            word,
		envs:            env,
		lazyEnvVars:     lazyEnvVars,
		pos:             0,
		escapeToken:     escapeToken,
		allowLazyExpand: allowLazyExpand,
	}
	sw.scanner.Init(strings.NewReader(word))
	outWord, _, err = sw.process()
	return outWord, sw.hasLazyExpand, err
}

// ProcessWords will use the 'env' list of environment variables,
// and replace any env var references in 'word' then it will also
// return a slice of strings which represents the 'word'
// split up based on spaces - taking into account quotes.  Note that
// this splitting is done **after** the env var substitutions are done.
// Note, each one is trimmed to remove leading and trailing spaces (unless
// they are quoted", but ProcessWord retains spaces between words.
// if allowLazyExpand is true, detects undeclared variables and don't replace it with empty string
func ProcessWords(word string, env, lazyEnvVars []string, escapeToken rune, allowLazyExpand bool) (words []string, isLazyExpanded bool, err error) {
	sw := &shellWord{
		word:            word,
		envs:            env,
		lazyEnvVars:     lazyEnvVars,
		pos:             0,
		escapeToken:     escapeToken,
		allowLazyExpand: allowLazyExpand,
	}
	sw.scanner.Init(strings.NewReader(word))
	_, words, err = sw.process()
	return words, false, err
}

func (sw *shellWord) process() (string, []string, error) {
	return sw.processStopOn(scanner.EOF)
}

type wordsStruct struct {
	word   string
	words  []string
	inWord bool
}

func (w *wordsStruct) addChar(ch rune) {
	if unicode.IsSpace(ch) && w.inWord {
		if len(w.word) != 0 {
			w.words = append(w.words, w.word)
			w.word = ""
			w.inWord = false
		}
	} else if !unicode.IsSpace(ch) {
		w.addRawChar(ch)
	}
}

func (w *wordsStruct) addRawChar(ch rune) {
	w.word += string(ch)
	w.inWord = true
}

func (w *wordsStruct) addString(str string) {
	var scan scanner.Scanner
	scan.Init(strings.NewReader(str))
	for scan.Peek() != scanner.EOF {
		w.addChar(scan.Next())
	}
}

func (w *wordsStruct) addRawString(str string) {
	w.word += str
	w.inWord = true
}

func (w *wordsStruct) getWords() []string {
	if len(w.word) > 0 {
		w.words = append(w.words, w.word)

		// Just in case we're called again by mistake
		w.word = ""
		w.inWord = false
	}
	return w.words
}

// Process the word, starting at 'pos', and stop when we get to the
// end of the word or the 'stopChar' character
func (sw *shellWord) processStopOn(stopChar rune) (string, []string, error) {
	var result string
	var words wordsStruct

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
				return "", []string{}, err
			}
			result += tmp

			if ch == rune('$') {
				words.addString(tmp)
			} else {
				words.addRawString(tmp)
			}
		} else {
			// Not special, just add it to the result
			ch = sw.scanner.Next()

			if ch == sw.escapeToken {
				// '\' (default escape token, but ` allowed) escapes, except end of line

				ch = sw.scanner.Next()

				if ch == scanner.EOF {
					break
				}

				words.addRawChar(ch)
			} else {
				words.addChar(ch)
			}

			result += string(ch)
		}
	}

	return result, words.getWords(), nil
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
	// But you can escape " with a \ (or ` if escape token set accordingly)
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
			if ch == sw.escapeToken {
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
			lookupResult := sw.getEnv(name)
			if !sw.allowLazyExpand && lookupResult.isLazyExpanded {
				return "", errors.New("can't reference a lazy expanded variable here")
			}
			if sw.allowLazyExpand {
				if !lookupResult.found || lookupResult.isLazyExpanded {
					sw.hasLazyExpand = true
				}
			}
			return lookupResult.value, nil
		}
		if ch == ':' {
			// Special ${xx:...} format processing
			// Yes it allows for recursive $'s in the ... spot

			sw.scanner.Next() // skip over :
			modifier := sw.scanner.Next()

			word, _, err := sw.processStopOn('}')
			if err != nil {
				return "", err
			}

			// Grab the current value of the variable in question so we
			// can use to to determine what to do based on the modifier
			lookupResult := sw.getEnv(name)
			if lookupResult.isLazyExpanded {
				return "", errors.New("Cannot substitute a lazy expanded variable")
			}
			if !lookupResult.found {
				lookupResult.value = ""
			}

			newValue := lookupResult.value

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
	lookupResult := sw.getEnv(name)
	if !sw.allowLazyExpand && lookupResult.isLazyExpanded {
		return "", errors.New("can't reference a lazy expanded variable here")
	}
	if sw.allowLazyExpand {
		if !lookupResult.found || lookupResult.isLazyExpanded {
			sw.hasLazyExpand = true
		}
	}
	return lookupResult.value, nil
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

type envResult struct {
	value          string
	found          bool
	isLazyExpanded bool
}

func (sw *shellWord) getEnv(name string) envResult {
	if runtime.GOOS == "windows" {
		// Case-insensitive environment variables on Windows
		name = strings.ToUpper(name)
	}
	for _, env := range sw.envs {
		i := strings.Index(env, "=")
		if i < 0 {
			if runtime.GOOS == "windows" {
				env = strings.ToUpper(env)
			}
			if name == env {
				// Should probably never get here, but just in case treat
				// it like "var" and "var=" are the same
				return envResult{"", true, sw.isEnvVarLazyExpanded(name)}
			}
			continue
		}
		compareName := env[:i]
		if runtime.GOOS == "windows" {
			compareName = strings.ToUpper(compareName)
		}
		if name != compareName {
			continue
		}
		return envResult{env[i+1:], true, sw.isEnvVarLazyExpanded(name)}
	}
	if sw.allowLazyExpand {
		return envResult{sw.lazyExpandedVariableReference(name), false, true}
	}
	return envResult{"", false, false}

}

func (sw *shellWord) isEnvVarLazyExpanded(name string) bool {
	if runtime.GOOS == "windows" {
		// Case-insensitive environment variables on Windows
		name = strings.ToUpper(name)
	}

	for _, known := range sw.lazyEnvVars {
		if runtime.GOOS == "windows" {
			known = strings.ToUpper(known)
		}
		if known == name {
			return true
		}
	}
	return false
}
