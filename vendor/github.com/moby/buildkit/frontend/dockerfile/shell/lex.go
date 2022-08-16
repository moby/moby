package shell

import (
	"bytes"
	"fmt"
	"strings"
	"text/scanner"
	"unicode"

	"github.com/pkg/errors"
)

// Lex performs shell word splitting and variable expansion.
//
// Lex takes a string and an array of env variables and
// process all quotes (" and ') as well as $xxx and ${xxx} env variable
// tokens.  Tries to mimic bash shell process.
// It doesn't support all flavors of ${xx:...} formats but new ones can
// be added by adding code to the "special ${} format processing" section
type Lex struct {
	escapeToken       rune
	RawQuotes         bool
	RawEscapes        bool
	SkipProcessQuotes bool
	SkipUnsetEnv      bool
}

// NewLex creates a new Lex which uses escapeToken to escape quotes.
func NewLex(escapeToken rune) *Lex {
	return &Lex{escapeToken: escapeToken}
}

// ProcessWord will use the 'env' list of environment variables,
// and replace any env var references in 'word'.
func (s *Lex) ProcessWord(word string, env []string) (string, error) {
	word, _, err := s.process(word, BuildEnvs(env))
	return word, err
}

// ProcessWords will use the 'env' list of environment variables,
// and replace any env var references in 'word' then it will also
// return a slice of strings which represents the 'word'
// split up based on spaces - taking into account quotes.  Note that
// this splitting is done **after** the env var substitutions are done.
// Note, each one is trimmed to remove leading and trailing spaces (unless
// they are quoted", but ProcessWord retains spaces between words.
func (s *Lex) ProcessWords(word string, env []string) ([]string, error) {
	_, words, err := s.process(word, BuildEnvs(env))
	return words, err
}

// ProcessWordWithMap will use the 'env' list of environment variables,
// and replace any env var references in 'word'.
func (s *Lex) ProcessWordWithMap(word string, env map[string]string) (string, error) {
	word, _, err := s.process(word, env)
	return word, err
}

// ProcessWordWithMatches will use the 'env' list of environment variables,
// replace any env var references in 'word' and return the env that were used.
func (s *Lex) ProcessWordWithMatches(word string, env map[string]string) (string, map[string]struct{}, error) {
	sw := s.init(word, env)
	word, _, err := sw.process(word)
	return word, sw.matches, err
}

func (s *Lex) ProcessWordsWithMap(word string, env map[string]string) ([]string, error) {
	_, words, err := s.process(word, env)
	return words, err
}

func (s *Lex) init(word string, env map[string]string) *shellWord {
	sw := &shellWord{
		envs:              env,
		escapeToken:       s.escapeToken,
		skipUnsetEnv:      s.SkipUnsetEnv,
		skipProcessQuotes: s.SkipProcessQuotes,
		rawQuotes:         s.RawQuotes,
		rawEscapes:        s.RawEscapes,
		matches:           make(map[string]struct{}),
	}
	sw.scanner.Init(strings.NewReader(word))
	return sw
}

func (s *Lex) process(word string, env map[string]string) (string, []string, error) {
	sw := s.init(word, env)
	return sw.process(word)
}

type shellWord struct {
	scanner           scanner.Scanner
	envs              map[string]string
	escapeToken       rune
	rawQuotes         bool
	rawEscapes        bool
	skipUnsetEnv      bool
	skipProcessQuotes bool
	matches           map[string]struct{}
}

func (sw *shellWord) process(source string) (string, []string, error) {
	word, words, err := sw.processStopOn(scanner.EOF)
	if err != nil {
		err = errors.Wrapf(err, "failed to process %q", source)
	}
	return word, words, err
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
	for _, ch := range str {
		w.addChar(ch)
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
	var result bytes.Buffer
	var words wordsStruct

	var charFuncMapping = map[rune]func() (string, error){
		'$': sw.processDollar,
	}
	if !sw.skipProcessQuotes {
		charFuncMapping['\''] = sw.processSingleQuote
		charFuncMapping['"'] = sw.processDoubleQuote
	}

	for sw.scanner.Peek() != scanner.EOF {
		ch := sw.scanner.Peek()

		if stopChar != scanner.EOF && ch == stopChar {
			sw.scanner.Next()
			return result.String(), words.getWords(), nil
		}
		if fn, ok := charFuncMapping[ch]; ok {
			// Call special processing func for certain chars
			tmp, err := fn()
			if err != nil {
				return "", []string{}, err
			}
			result.WriteString(tmp)

			if ch == rune('$') {
				words.addString(tmp)
			} else {
				words.addRawString(tmp)
			}
		} else {
			// Not special, just add it to the result
			ch = sw.scanner.Next()

			if ch == sw.escapeToken {
				if sw.rawEscapes {
					words.addRawChar(ch)
					result.WriteRune(ch)
				}

				// '\' (default escape token, but ` allowed) escapes, except end of line
				ch = sw.scanner.Next()

				if ch == scanner.EOF {
					break
				}

				words.addRawChar(ch)
			} else {
				words.addChar(ch)
			}

			result.WriteRune(ch)
		}
	}
	if stopChar != scanner.EOF {
		return "", []string{}, errors.Errorf("unexpected end of statement while looking for matching %s", string(stopChar))
	}
	return result.String(), words.getWords(), nil
}

func (sw *shellWord) processSingleQuote() (string, error) {
	// All chars between single quotes are taken as-is
	// Note, you can't escape '
	//
	// From the "sh" man page:
	// Single Quotes
	//   Enclosing characters in single quotes preserves the literal meaning of
	//   all the characters (except single quotes, making it impossible to put
	//   single-quotes in a single-quoted string).

	var result bytes.Buffer

	ch := sw.scanner.Next()
	if sw.rawQuotes {
		result.WriteRune(ch)
	}

	for {
		ch = sw.scanner.Next()
		switch ch {
		case scanner.EOF:
			return "", errors.New("unexpected end of statement while looking for matching single-quote")
		case '\'':
			if sw.rawQuotes {
				result.WriteRune(ch)
			}
			return result.String(), nil
		}
		result.WriteRune(ch)
	}
}

func (sw *shellWord) processDoubleQuote() (string, error) {
	// All chars up to the next " are taken as-is, even ', except any $ chars
	// But you can escape " with a \ (or ` if escape token set accordingly)
	//
	// From the "sh" man page:
	// Double Quotes
	//  Enclosing characters within double quotes preserves the literal meaning
	//  of all characters except dollarsign ($), backquote (`), and backslash
	//  (\).  The backslash inside double quotes is historically weird, and
	//  serves to quote only the following characters:
	//    $ ` " \ <newline>.
	//  Otherwise it remains literal.

	var result bytes.Buffer

	ch := sw.scanner.Next()
	if sw.rawQuotes {
		result.WriteRune(ch)
	}

	for {
		switch sw.scanner.Peek() {
		case scanner.EOF:
			return "", errors.New("unexpected end of statement while looking for matching double-quote")
		case '"':
			ch := sw.scanner.Next()
			if sw.rawQuotes {
				result.WriteRune(ch)
			}
			return result.String(), nil
		case '$':
			value, err := sw.processDollar()
			if err != nil {
				return "", err
			}
			result.WriteString(value)
		default:
			ch := sw.scanner.Next()
			if ch == sw.escapeToken {
				if sw.rawEscapes {
					result.WriteRune(ch)
				}

				switch sw.scanner.Peek() {
				case scanner.EOF:
					// Ignore \ at end of word
					continue
				case '"', '$', sw.escapeToken:
					// These chars can be escaped, all other \'s are left as-is
					// Note: for now don't do anything special with ` chars.
					// Not sure what to do with them anyway since we're not going
					// to execute the text in there (not now anyway).
					ch = sw.scanner.Next()
				}
			}
			result.WriteRune(ch)
		}
	}
}

func (sw *shellWord) processDollar() (string, error) {
	sw.scanner.Next()

	// $xxx case
	if sw.scanner.Peek() != '{' {
		name := sw.processName()
		if name == "" {
			return "$", nil
		}
		value, found := sw.getEnv(name)
		if !found && sw.skipUnsetEnv {
			return "$" + name, nil
		}
		return value, nil
	}

	sw.scanner.Next()
	switch sw.scanner.Peek() {
	case scanner.EOF:
		return "", errors.New("syntax error: missing '}'")
	case '{', '}', ':':
		// Invalid ${{xx}, ${:xx}, ${:}. ${} case
		return "", errors.New("syntax error: bad substitution")
	}
	name := sw.processName()
	ch := sw.scanner.Next()
	switch ch {
	case '}':
		// Normal ${xx} case
		value, found := sw.getEnv(name)
		if !found && sw.skipUnsetEnv {
			return fmt.Sprintf("${%s}", name), nil
		}
		return value, nil
	case '?':
		word, _, err := sw.processStopOn('}')
		if err != nil {
			if sw.scanner.Peek() == scanner.EOF {
				return "", errors.New("syntax error: missing '}'")
			}
			return "", err
		}
		newValue, found := sw.getEnv(name)
		if !found {
			if sw.skipUnsetEnv {
				return fmt.Sprintf("${%s?%s}", name, word), nil
			}
			message := "is not allowed to be unset"
			if word != "" {
				message = word
			}
			return "", errors.Errorf("%s: %s", name, message)
		}
		return newValue, nil
	case ':':
		// Special ${xx:...} format processing
		// Yes it allows for recursive $'s in the ... spot
		modifier := sw.scanner.Next()

		word, _, err := sw.processStopOn('}')
		if err != nil {
			if sw.scanner.Peek() == scanner.EOF {
				return "", errors.New("syntax error: missing '}'")
			}
			return "", err
		}

		// Grab the current value of the variable in question so we
		// can use to to determine what to do based on the modifier
		newValue, found := sw.getEnv(name)

		switch modifier {
		case '+':
			if newValue != "" {
				newValue = word
			}
			if !found && sw.skipUnsetEnv {
				return fmt.Sprintf("${%s:%s%s}", name, string(modifier), word), nil
			}
			return newValue, nil

		case '-':
			if newValue == "" {
				newValue = word
			}
			if !found && sw.skipUnsetEnv {
				return fmt.Sprintf("${%s:%s%s}", name, string(modifier), word), nil
			}

			return newValue, nil

		case '?':
			if !found {
				if sw.skipUnsetEnv {
					return fmt.Sprintf("${%s:%s%s}", name, string(modifier), word), nil
				}
				message := "is not allowed to be unset"
				if word != "" {
					message = word
				}
				return "", errors.Errorf("%s: %s", name, message)
			}
			if newValue == "" {
				message := "is not allowed to be empty"
				if word != "" {
					message = word
				}
				return "", errors.Errorf("%s: %s", name, message)
			}
			return newValue, nil

		default:
			return "", errors.Errorf("unsupported modifier (%c) in substitution", modifier)
		}
	}
	return "", errors.Errorf("missing ':' in substitution")
}

func (sw *shellWord) processName() string {
	// Read in a name (alphanumeric or _)
	// If it starts with a numeric then just return $#
	var name bytes.Buffer

	for sw.scanner.Peek() != scanner.EOF {
		ch := sw.scanner.Peek()
		if name.Len() == 0 && unicode.IsDigit(ch) {
			for sw.scanner.Peek() != scanner.EOF && unicode.IsDigit(sw.scanner.Peek()) {
				// Keep reading until the first non-digit character, or EOF
				ch = sw.scanner.Next()
				name.WriteRune(ch)
			}
			return name.String()
		}
		if name.Len() == 0 && isSpecialParam(ch) {
			ch = sw.scanner.Next()
			return string(ch)
		}
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			break
		}
		ch = sw.scanner.Next()
		name.WriteRune(ch)
	}

	return name.String()
}

// isSpecialParam checks if the provided character is a special parameters,
// as defined in http://pubs.opengroup.org/onlinepubs/009695399/utilities/xcu_chap02.html#tag_02_05_02
func isSpecialParam(char rune) bool {
	switch char {
	case '@', '*', '#', '?', '-', '$', '!', '0':
		// Special parameters
		// http://pubs.opengroup.org/onlinepubs/009695399/utilities/xcu_chap02.html#tag_02_05_02
		return true
	}
	return false
}

func (sw *shellWord) getEnv(name string) (string, bool) {
	for key, value := range sw.envs {
		if EqualEnvKeys(name, key) {
			sw.matches[name] = struct{}{}
			return value, true
		}
	}
	return "", false
}

func BuildEnvs(env []string) map[string]string {
	envs := map[string]string{}

	for _, e := range env {
		i := strings.Index(e, "=")

		if i < 0 {
			envs[e] = ""
		} else {
			k := e[:i]
			v := e[i+1:]

			// overwrite value if key already exists
			envs[k] = v
		}
	}

	return envs
}
