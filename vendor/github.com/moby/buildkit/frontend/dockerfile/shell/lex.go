package shell

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"text/scanner"
	"unicode"

	"github.com/pkg/errors"
)

type EnvGetter interface {
	Get(string) (string, bool)
	Keys() []string
}

// Lex performs shell word splitting and variable expansion.
//
// Lex takes a string and an array of env variables and
// process all quotes (" and ') as well as $xxx and ${xxx} env variable
// tokens.  Tries to mimic bash shell process.
// It doesn't support all flavors of ${xx:...} formats but new ones can
// be added by adding code to the "special ${} format processing" section
//
// It is not safe to call methods on a Lex instance concurrently.
type Lex struct {
	escapeToken       rune
	RawQuotes         bool
	RawEscapes        bool
	SkipProcessQuotes bool
	SkipUnsetEnv      bool
	shellWord         shellWord
}

// NewLex creates a new Lex which uses escapeToken to escape quotes.
func NewLex(escapeToken rune) *Lex {
	return &Lex{escapeToken: escapeToken}
}

// ProcessWord will use the 'env' list of environment variables,
// and replace any env var references in 'word'. It will also
// return variables in word which were not found in the 'env' list,
// which is useful in later linting.
// TODO: rename
func (s *Lex) ProcessWord(word string, env EnvGetter) (string, map[string]struct{}, error) {
	result, err := s.process(word, env, true)
	return result.Result, result.Unmatched, err
}

// ProcessWords will use the 'env' list of environment variables,
// and replace any env var references in 'word' then it will also
// return a slice of strings which represents the 'word'
// split up based on spaces - taking into account quotes.  Note that
// this splitting is done **after** the env var substitutions are done.
// Note, each one is trimmed to remove leading and trailing spaces (unless
// they are quoted", but ProcessWord retains spaces between words.
func (s *Lex) ProcessWords(word string, env EnvGetter) ([]string, error) {
	result, err := s.process(word, env, false)
	return result.Words, err
}

type ProcessWordResult struct {
	Result    string
	Words     []string
	Matched   map[string]struct{}
	Unmatched map[string]struct{}
}

// ProcessWordWithMatches will use the 'env' list of environment variables,
// replace any env var references in 'word' and return the env that were used.
func (s *Lex) ProcessWordWithMatches(word string, env EnvGetter) (ProcessWordResult, error) {
	return s.process(word, env, true)
}

func (s *Lex) initWord(word string, env EnvGetter, capture bool) *shellWord {
	sw := &s.shellWord
	sw.Lex = s
	sw.envs = env
	sw.capture = capture
	sw.rawEscapes = s.RawEscapes
	if capture {
		sw.matches = nil
		sw.nonmatches = nil
	}
	sw.scanner.Init(strings.NewReader(word))
	return sw
}

func (s *Lex) process(word string, env EnvGetter, capture bool) (ProcessWordResult, error) {
	sw := s.initWord(word, env, capture)
	word, words, err := sw.process(word)
	return ProcessWordResult{
		Result:    word,
		Words:     words,
		Matched:   sw.matches,
		Unmatched: sw.nonmatches,
	}, err
}

type shellWord struct {
	*Lex
	wordsBuffer strings.Builder
	scanner     scanner.Scanner
	envs        EnvGetter
	rawEscapes  bool
	capture     bool // capture matches and nonmatches
	matches     map[string]struct{}
	nonmatches  map[string]struct{}
}

func (sw *shellWord) process(source string) (string, []string, error) {
	word, words, err := sw.processStopOn(scanner.EOF, sw.rawEscapes)
	if err != nil {
		err = errors.Wrapf(err, "failed to process %q", source)
	}
	return word, words, err
}

type wordsStruct struct {
	buf    *strings.Builder
	words  []string
	inWord bool
}

func (w *wordsStruct) addChar(ch rune) {
	if unicode.IsSpace(ch) && w.inWord {
		if w.buf.Len() != 0 {
			w.words = append(w.words, w.buf.String())
			w.buf.Reset()
			w.inWord = false
		}
	} else if !unicode.IsSpace(ch) {
		w.addRawChar(ch)
	}
}

func (w *wordsStruct) addRawChar(ch rune) {
	w.buf.WriteRune(ch)
	w.inWord = true
}

func (w *wordsStruct) addString(str string) {
	for _, ch := range str {
		w.addChar(ch)
	}
}

func (w *wordsStruct) addRawString(str string) {
	w.buf.WriteString(str)
	w.inWord = true
}

func (w *wordsStruct) getWords() []string {
	if w.buf.Len() > 0 {
		w.words = append(w.words, w.buf.String())

		// Just in case we're called again by mistake
		w.buf.Reset()
		w.inWord = false
	}
	return w.words
}

// Process the word, starting at 'pos', and stop when we get to the
// end of the word or the 'stopChar' character
func (sw *shellWord) processStopOn(stopChar rune, rawEscapes bool) (string, []string, error) {
	// result buffer can't be currently shared for shellWord as it is called internally
	// by processDollar
	var result strings.Builder
	sw.wordsBuffer.Reset()
	var words wordsStruct
	words.buf = &sw.wordsBuffer

	// no need to initialize all the time
	var charFuncMapping = map[rune]func() (string, error){
		'$': sw.processDollar,
	}
	if !sw.SkipProcessQuotes {
		charFuncMapping['\''] = sw.processSingleQuote
		charFuncMapping['"'] = sw.processDoubleQuote
	}

	// temporarily set sw.rawEscapes if needed
	if rawEscapes != sw.rawEscapes {
		sw.rawEscapes = rawEscapes
		defer func() {
			sw.rawEscapes = !rawEscapes
		}()
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
	if sw.RawQuotes {
		result.WriteRune(ch)
	}

	for {
		ch = sw.scanner.Next()
		switch ch {
		case scanner.EOF:
			return "", errors.New("unexpected end of statement while looking for matching single-quote")
		case '\'':
			if sw.RawQuotes {
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
	if sw.RawQuotes {
		result.WriteRune(ch)
	}

	for {
		switch sw.scanner.Peek() {
		case scanner.EOF:
			return "", errors.New("unexpected end of statement while looking for matching double-quote")
		case '"':
			ch := sw.scanner.Next()
			if sw.RawQuotes {
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
		if !found && sw.SkipUnsetEnv {
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
	chs := string(ch)
	nullIsUnset := false

	switch ch {
	case '}':
		// Normal ${xx} case
		value, set := sw.getEnv(name)
		if !set && sw.SkipUnsetEnv {
			return fmt.Sprintf("${%s}", name), nil
		}
		return value, nil
	case ':':
		nullIsUnset = true
		ch = sw.scanner.Next()
		chs += string(ch)
		fallthrough
	case '+', '-', '?', '#', '%':
		rawEscapes := ch == '#' || ch == '%'
		if nullIsUnset && rawEscapes {
			return "", errors.Errorf("unsupported modifier (%s) in substitution", chs)
		}
		word, _, err := sw.processStopOn('}', rawEscapes)
		if err != nil {
			if sw.scanner.Peek() == scanner.EOF {
				return "", errors.New("syntax error: missing '}'")
			}
			return "", err
		}

		// Grab the current value of the variable in question so we
		// can use it to determine what to do based on the modifier
		value, set := sw.getEnv(name)
		if sw.SkipUnsetEnv && !set {
			return fmt.Sprintf("${%s%s%s}", name, chs, word), nil
		}

		switch ch {
		case '-':
			if !set || (nullIsUnset && value == "") {
				return word, nil
			}
			return value, nil
		case '+':
			if !set || (nullIsUnset && value == "") {
				return "", nil
			}
			return word, nil
		case '?':
			if !set {
				message := "is not allowed to be unset"
				if word != "" {
					message = word
				}
				return "", errors.Errorf("%s: %s", name, message)
			}
			if nullIsUnset && value == "" {
				message := "is not allowed to be empty"
				if word != "" {
					message = word
				}
				return "", errors.Errorf("%s: %s", name, message)
			}
			return value, nil
		case '%', '#':
			// %/# matches the shortest pattern expansion, %%/## the longest
			greedy := false

			if len(word) > 0 && word[0] == byte(ch) {
				greedy = true
				word = word[1:]
			}

			if ch == '%' {
				return trimSuffix(word, value, greedy)
			}
			return trimPrefix(word, value, greedy)
		default:
			return "", errors.Errorf("unsupported modifier (%s) in substitution", chs)
		}
	case '/':
		replaceAll := sw.scanner.Peek() == '/'
		if replaceAll {
			sw.scanner.Next()
		}

		pattern, _, err := sw.processStopOn('/', true)
		if err != nil {
			if sw.scanner.Peek() == scanner.EOF {
				return "", errors.New("syntax error: missing '/' in ${}")
			}
			return "", err
		}

		replacement, _, err := sw.processStopOn('}', true)
		if err != nil {
			if sw.scanner.Peek() == scanner.EOF {
				return "", errors.New("syntax error: missing '}'")
			}
			return "", err
		}

		value, set := sw.getEnv(name)
		if sw.SkipUnsetEnv && !set {
			return fmt.Sprintf("${%s/%s/%s}", name, pattern, replacement), nil
		}

		re, err := convertShellPatternToRegex(pattern, true, false)
		if err != nil {
			return "", errors.Errorf("invalid pattern (%s) in substitution: %s", pattern, err)
		}
		if replaceAll {
			value = re.ReplaceAllString(value, replacement)
		} else {
			if idx := re.FindStringIndex(value); idx != nil {
				value = value[0:idx[0]] + replacement + value[idx[1]:]
			}
		}
		return value, nil
	default:
		return "", errors.Errorf("unsupported modifier (%s) in substitution", chs)
	}
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
	v, ok := sw.envs.Get(name)
	if ok {
		if sw.capture {
			if sw.matches == nil {
				sw.matches = make(map[string]struct{})
			}
			sw.matches[name] = struct{}{}
		}
		return v, true
	}
	if sw.capture {
		if sw.nonmatches == nil {
			sw.nonmatches = make(map[string]struct{})
		}
		sw.nonmatches[name] = struct{}{}
	}
	return "", false
}

func EnvsFromSlice(env []string) EnvGetter {
	envs := map[string]string{}
	keys := make([]string, 0, len(env))
	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		keys = append(keys, k)
		envs[NormalizeEnvKey(k)] = v
	}
	return &envGetter{env: envs, keys: keys}
}

type envGetter struct {
	env  map[string]string
	keys []string
}

var _ EnvGetter = &envGetter{}

func (e *envGetter) Get(key string) (string, bool) {
	key = NormalizeEnvKey(key)
	v, ok := e.env[key]
	return v, ok
}

func (e *envGetter) Keys() []string {
	return e.keys
}

// convertShellPatternToRegex converts a shell-like wildcard pattern
// (? is a single char, * either the shortest or longest (greedy) string)
// to an equivalent regular expression.
//
// Based on
// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_13
// but without the bracket expressions (`[]`)
func convertShellPatternToRegex(pattern string, greedy bool, anchored bool) (*regexp.Regexp, error) {
	var s scanner.Scanner
	s.Init(strings.NewReader(pattern))
	var out strings.Builder
	out.Grow(len(pattern) + 4)

	// match only at the beginning of the string
	if anchored {
		out.WriteByte('^')
	}

	// default: non-greedy wildcards
	starPattern := ".*?"
	if greedy {
		starPattern = ".*"
	}

	for tok := s.Next(); tok != scanner.EOF; tok = s.Next() {
		switch tok {
		case '*':
			out.WriteString(starPattern)
			continue
		case '?':
			out.WriteByte('.')
			continue
		case '\\':
			// } and / as part of ${} need to be escaped, but the escape isn't part
			// of the pattern
			if s.Peek() == '}' || s.Peek() == '/' {
				continue
			}
			out.WriteRune('\\')
			tok = s.Next()
			if tok != '*' && tok != '?' && tok != '\\' {
				return nil, errors.Errorf("invalid escape '\\%c'", tok)
			}
		// regex characters that need to be escaped
		// escaping closing is optional, but done for consistency
		case '[', ']', '{', '}', '.', '+', '(', ')', '|', '^', '$':
			out.WriteByte('\\')
		}
		out.WriteRune(tok)
	}
	return regexp.Compile(out.String())
}

func trimPrefix(word, value string, greedy bool) (string, error) {
	re, err := convertShellPatternToRegex(word, greedy, true)
	if err != nil {
		return "", errors.Errorf("invalid pattern (%s) in substitution: %s", word, err)
	}

	if idx := re.FindStringIndex(value); idx != nil {
		value = value[idx[1]:]
	}
	return value, nil
}

// reverse without avoid reversing escapes, i.e. a\*c -> c\*a
func reversePattern(pattern string) string {
	patternRunes := []rune(pattern)
	out := make([]rune, len(patternRunes))
	lastIdx := len(patternRunes) - 1
	for i := 0; i <= lastIdx; {
		tok := patternRunes[i]
		outIdx := lastIdx - i
		if tok == '\\' && i != lastIdx {
			out[outIdx-1] = tok
			// the pattern is taken from a ${var#pattern}, so the last
			// character can't be an escape character
			out[outIdx] = patternRunes[i+1]
			i += 2
		} else {
			out[outIdx] = tok
			i++
		}
	}
	return string(out)
}

func reverseString(str string) string {
	out := []rune(str)
	slices.Reverse(out)
	return string(out)
}

func trimSuffix(pattern, word string, greedy bool) (string, error) {
	// regular expressions can't handle finding the shortest rightmost
	// string so we reverse both search space and pattern to convert it
	// to a leftmost search in both cases
	pattern = reversePattern(pattern)
	word = reverseString(word)
	str, err := trimPrefix(pattern, word, greedy)
	if err != nil {
		return "", err
	}
	return reverseString(str), nil
}
