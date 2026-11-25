package pattern

import (
	"path"
	"unicode/utf8"
)

var ErrBadPattern = path.ErrBadPattern

// Match reports whether name matches the shell pattern.
// The pattern syntax is:
//
//	pattern:
//	  { term }
//	term:
//	  '*'         matches any sequence of non-path-separators
//	  '/**/'      matches zero or more directories
//	  '?'         matches any single non-path-separator character
//	  '[' [ '^' '!' ] { character-range } ']'
//	              character class (must be non-empty)
//	              starting with `^` or `!` negates the class
//	  '{' { term } [ ',' { term } ... ] '}'
//	              alternatives
//	  c           matches character c (c != '*', '?', '\\', '[')
//	  '\\' c      matches character c
//
//	character-range:
//	  c           matches character c (c != '\\', '-', ']')
//	  '\\' c      matches character c
//	  lo '-' hi   matches character c for lo <= c <= hi
//
// Match returns true if `name` matches the file name `pattern`. `name` and
// `pattern` are split on forward slash (`/`) characters and may be relative or
// absolute.
//
// Match requires pattern to match all of name, not just a substring.
// The only possible returned error is ErrBadPattern, when pattern
// is malformed.
//
// A doublestar (`**`) should appear surrounded by path separators such as
// `/**/`.  A mid-pattern doublestar (`**`) behaves like bash's globstar
// option: a pattern such as `path/to/**.txt` would return the same results as
// `path/to/*.txt`. The pattern you're looking for is `path/to/**/*.txt`.
//
// Note: this is meant as a drop-in replacement for path.Match() which
// always uses '/' as the path separator. If you want to support systems
// which use a different path separator (such as Windows), what you want
// is PathMatch(). Alternatively, you can run filepath.ToSlash() on both
// pattern and name and then use this function.
//
// Note: users should _not_ count on the returned error,
// doublestar.ErrBadPattern, being equal to path.ErrBadPattern.
func Match(pattern, name string) (bool, error) {
	return matchWithSeparator(pattern, name, '/', true)
}

func matchWithSeparator(pattern, name string, separator rune, validate bool) (matched bool, err error) {
	return doMatchWithSeparator(pattern, name, separator, validate, -1, -1, -1, -1, 0, 0)
}

func doMatchWithSeparator(pattern, name string, separator rune, validate bool, doublestarPatternBacktrack, doublestarNameBacktrack, starPatternBacktrack, starNameBacktrack, patIdx, nameIdx int) (matched bool, err error) {
	patLen := len(pattern)
	nameLen := len(name)
	startOfSegment := true
MATCH:
	for nameIdx < nameLen {
		if patIdx < patLen {
			switch pattern[patIdx] {
			case '*':
				if patIdx++; patIdx < patLen && pattern[patIdx] == '*' {
					// doublestar - must begin with a path separator, otherwise we'll
					// treat it like a single star like bash
					patIdx++
					if startOfSegment {
						if patIdx >= patLen {
							// pattern ends in `/**`: return true
							return true, nil
						}

						// doublestar must also end with a path separator, otherwise we're
						// just going to treat the doublestar as a single star like bash
						patRune, patRuneLen := utf8.DecodeRuneInString(pattern[patIdx:])
						if patRune == separator {
							patIdx += patRuneLen

							doublestarPatternBacktrack = patIdx
							doublestarNameBacktrack = nameIdx
							starPatternBacktrack = -1
							starNameBacktrack = -1
							continue
						}
					}
				}
				startOfSegment = false

				starPatternBacktrack = patIdx
				starNameBacktrack = nameIdx
				continue

			case '?':
				startOfSegment = false
				nameRune, nameRuneLen := utf8.DecodeRuneInString(name[nameIdx:])
				if nameRune == separator {
					// `?` cannot match the separator
					break
				}

				patIdx++
				nameIdx += nameRuneLen
				continue

			case '[':
				startOfSegment = false
				if patIdx++; patIdx >= patLen {
					// class didn't end
					return false, ErrBadPattern
				}
				nameRune, nameRuneLen := utf8.DecodeRuneInString(name[nameIdx:])

				matched := false
				negate := pattern[patIdx] == '!' || pattern[patIdx] == '^'
				if negate {
					patIdx++
				}

				if patIdx >= patLen || pattern[patIdx] == ']' {
					// class didn't end or empty character class
					return false, ErrBadPattern
				}

				last := utf8.MaxRune
				for patIdx < patLen && pattern[patIdx] != ']' {
					patRune, patRuneLen := utf8.DecodeRuneInString(pattern[patIdx:])
					patIdx += patRuneLen

					// match a range
					if last < utf8.MaxRune && patRune == '-' && patIdx < patLen && pattern[patIdx] != ']' {
						if pattern[patIdx] == '\\' {
							// next character is escaped
							patIdx++
						}
						patRune, patRuneLen = utf8.DecodeRuneInString(pattern[patIdx:])
						patIdx += patRuneLen

						if last <= nameRune && nameRune <= patRune {
							matched = true
							break
						}

						// didn't match range - reset `last`
						last = utf8.MaxRune
						continue
					}

					// not a range - check if the next rune is escaped
					if patRune == '\\' {
						patRune, patRuneLen = utf8.DecodeRuneInString(pattern[patIdx:])
						patIdx += patRuneLen
					}

					// check if the rune matches
					if patRune == nameRune {
						matched = true
						break
					}

					// no matches yet
					last = patRune
				}

				if matched == negate {
					// failed to match - if we reached the end of the pattern, that means
					// we never found a closing `]`
					if patIdx >= patLen {
						return false, ErrBadPattern
					}
					break
				}

				closingIdx := indexUnescapedByte(pattern[patIdx:], ']', true)
				if closingIdx == -1 {
					// no closing `]`
					return false, ErrBadPattern
				}

				patIdx += closingIdx + 1
				nameIdx += nameRuneLen
				continue

			case '{':
				startOfSegment = false
				beforeIdx := patIdx
				patIdx++
				closingIdx := indexMatchedClosingAlt(pattern[patIdx:], separator != '\\')
				if closingIdx == -1 {
					// no closing `}`
					return false, ErrBadPattern
				}
				closingIdx += patIdx

				for {
					commaIdx := indexNextAlt(pattern[patIdx:closingIdx], separator != '\\')
					if commaIdx == -1 {
						break
					}
					commaIdx += patIdx

					result, err := doMatchWithSeparator(pattern[:beforeIdx]+pattern[patIdx:commaIdx]+pattern[closingIdx+1:], name, separator, validate, doublestarPatternBacktrack, doublestarNameBacktrack, starPatternBacktrack, starNameBacktrack, beforeIdx, nameIdx)
					if result || err != nil {
						return result, err
					}

					patIdx = commaIdx + 1
				}
				return doMatchWithSeparator(pattern[:beforeIdx]+pattern[patIdx:closingIdx]+pattern[closingIdx+1:], name, separator, validate, doublestarPatternBacktrack, doublestarNameBacktrack, starPatternBacktrack, starNameBacktrack, beforeIdx, nameIdx)

			case '\\':
				if separator != '\\' {
					// next rune is "escaped" in the pattern - literal match
					if patIdx++; patIdx >= patLen {
						// pattern ended
						return false, ErrBadPattern
					}
				}
				fallthrough

			default:
				patRune, patRuneLen := utf8.DecodeRuneInString(pattern[patIdx:])
				nameRune, nameRuneLen := utf8.DecodeRuneInString(name[nameIdx:])
				if patRune != nameRune {
					if separator != '\\' && patIdx > 0 && pattern[patIdx-1] == '\\' {
						// if this rune was meant to be escaped, we need to move patIdx
						// back to the backslash before backtracking or validating below
						patIdx--
					}
					break
				}

				patIdx += patRuneLen
				nameIdx += nameRuneLen
				startOfSegment = patRune == separator
				continue
			}
		}

		if starPatternBacktrack >= 0 {
			// `*` backtrack, but only if the `name` rune isn't the separator
			nameRune, nameRuneLen := utf8.DecodeRuneInString(name[starNameBacktrack:])
			if nameRune != separator {
				starNameBacktrack += nameRuneLen
				patIdx = starPatternBacktrack
				nameIdx = starNameBacktrack
				startOfSegment = false
				continue
			}
		}

		if doublestarPatternBacktrack >= 0 {
			// `**` backtrack, advance `name` past next separator
			nameIdx = doublestarNameBacktrack
			for nameIdx < nameLen {
				nameRune, nameRuneLen := utf8.DecodeRuneInString(name[nameIdx:])
				nameIdx += nameRuneLen
				if nameRune == separator {
					doublestarNameBacktrack = nameIdx
					patIdx = doublestarPatternBacktrack
					startOfSegment = true
					continue MATCH
				}
			}
		}

		if validate && patIdx < patLen && !doValidatePattern(pattern[patIdx:], separator) {
			return false, ErrBadPattern
		}
		return false, nil
	}

	if nameIdx < nameLen {
		// we reached the end of `pattern` before the end of `name`
		return false, nil
	}

	// we've reached the end of `name`; we've successfully matched if we've also
	// reached the end of `pattern`, or if the rest of `pattern` can match a
	// zero-length string
	return isZeroLengthPattern(pattern[patIdx:], separator)
}

func isZeroLengthPattern(pattern string, separator rune) (ret bool, err error) {
	// `/**`, `**/`, and `/**/` are special cases - a pattern such as `path/to/a/**` or `path/to/a/**/`
	// *should* match `path/to/a` because `a` might be a directory
	if pattern == "" ||
		pattern == "*" ||
		pattern == "**" ||
		pattern == string(separator)+"**" ||
		pattern == "**"+string(separator) ||
		pattern == string(separator)+"**"+string(separator) {
		return true, nil
	}

	if pattern[0] == '{' {
		closingIdx := indexMatchedClosingAlt(pattern[1:], separator != '\\')
		if closingIdx == -1 {
			// no closing '}'
			return false, ErrBadPattern
		}
		closingIdx += 1

		patIdx := 1
		for {
			commaIdx := indexNextAlt(pattern[patIdx:closingIdx], separator != '\\')
			if commaIdx == -1 {
				break
			}
			commaIdx += patIdx

			ret, err = isZeroLengthPattern(pattern[patIdx:commaIdx]+pattern[closingIdx+1:], separator)
			if ret || err != nil {
				return
			}

			patIdx = commaIdx + 1
		}
		return isZeroLengthPattern(pattern[patIdx:closingIdx]+pattern[closingIdx+1:], separator)
	}

	// no luck - validate the rest of the pattern
	if !doValidatePattern(pattern, separator) {
		return false, ErrBadPattern
	}
	return false, nil
}

// Finds the next comma, but ignores any commas that appear inside nested `{}`.
// Assumes that each opening bracket has a corresponding closing bracket.
func indexNextAlt(s string, allowEscaping bool) int {
	alts := 1
	l := len(s)
	for i := 0; i < l; i++ {
		if allowEscaping && s[i] == '\\' {
			// skip next byte
			i++
		} else if s[i] == '{' {
			alts++
		} else if s[i] == '}' {
			alts--
		} else if s[i] == ',' && alts == 1 {
			return i
		}
	}
	return -1
}

// Finds the index of the first unescaped byte `c`, or negative 1.
func indexUnescapedByte(s string, c byte, allowEscaping bool) int {
	l := len(s)
	for i := 0; i < l; i++ {
		if allowEscaping && s[i] == '\\' {
			// skip next byte
			i++
		} else if s[i] == c {
			return i
		}
	}
	return -1
}

// Assuming the byte before the beginning of `s` is an opening `{`, this
// function will find the index of the matching `}`. That is, it'll skip over
// any nested `{}` and account for escaping
func indexMatchedClosingAlt(s string, allowEscaping bool) int {
	alts := 1
	l := len(s)
	for i := 0; i < l; i++ {
		if allowEscaping && s[i] == '\\' {
			// skip next byte
			i++
		} else if s[i] == '{' {
			alts++
		} else if s[i] == '}' {
			if alts--; alts == 0 {
				return i
			}
		}
	}
	return -1
}

// Validate a pattern. Patterns are validated while they run in Match(),
// PathMatch(), and Glob(), so, you normally wouldn't need to call this.
// However, there are cases where this might be useful: for example, if your
// program allows a user to enter a pattern that you'll run at a later time,
// you might want to validate it.
//
// ValidatePattern assumes your pattern uses '/' as the path separator.
func ValidatePattern(s string) bool {
	return doValidatePattern(s, '/')
}

func doValidatePattern(s string, separator rune) bool {
	altDepth := 0
	l := len(s)
VALIDATE:
	for i := 0; i < l; i++ {
		switch s[i] {
		case '\\':
			if separator != '\\' {
				// skip the next byte - return false if there is no next byte
				if i++; i >= l {
					return false
				}
			}
			continue

		case '[':
			if i++; i >= l {
				// class didn't end
				return false
			}
			if s[i] == '^' || s[i] == '!' {
				i++
			}
			if i >= l || s[i] == ']' {
				// class didn't end or empty character class
				return false
			}

			for ; i < l; i++ {
				if separator != '\\' && s[i] == '\\' {
					i++
				} else if s[i] == ']' {
					// looks good
					continue VALIDATE
				}
			}

			// class didn't end
			return false

		case '{':
			altDepth++
			continue

		case '}':
			if altDepth == 0 {
				// alt end without a corresponding start
				return false
			}
			altDepth--
			continue
		}
	}

	// valid as long as all alts are closed
	return altDepth == 0
}
