// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package mangling

import (
	"unicode"
)

type splitterOption func(*splitter)

// withPostSplitInitialismCheck allows to catch initialisms after main split process
func withPostSplitInitialismCheck(s *splitter) {
	s.postSplitInitialismCheck = true
}

func withReplaceFunc(fn ReplaceFunc) func(*splitter) {
	return func(s *splitter) {
		s.replaceFunc = fn
	}
}

func withInitialismsCache(c *initialismsCache) splitterOption {
	return func(s *splitter) {
		s.initialismsCache = c
	}
}

type (
	initialismMatch struct {
		body       []rune
		start, end int
		complete   bool
		hasPlural  pluralForm
	}
	initialismMatches []initialismMatch
)

func (m initialismMatch) isZero() bool {
	return m.start == 0 && m.end == 0
}

type splitter struct {
	*initialismsCache

	postSplitInitialismCheck bool
	replaceFunc              ReplaceFunc
}

func newSplitter(options ...splitterOption) splitter {
	var s splitter

	for _, option := range options {
		option(&s)
	}

	if s.replaceFunc == nil {
		s.replaceFunc = defaultReplaceTable
	}

	return s
}

func (s splitter) split(name string) *[]nameLexem {
	nameRunes := []rune(name)
	matches := s.gatherInitialismMatches(nameRunes)
	if matches == nil {
		return poolOfLexems.BorrowLexems()
	}

	return s.mapMatchesToNameLexems(nameRunes, matches)
}

func (s splitter) gatherInitialismMatches(nameRunes []rune) *initialismMatches {
	var matches *initialismMatches

	for currentRunePosition, currentRune := range nameRunes {
		// recycle these allocations as we loop over runes
		// with such recycling, only 2 slices should be allocated per call
		// instead of o(n).
		newMatches := poolOfMatches.BorrowMatches()

		// check current initialism matches
		if matches != nil { // skip first iteration
			for _, match := range *matches {
				if keepCompleteMatch := match.complete; keepCompleteMatch {
					*newMatches = append(*newMatches, match)

					// the match is complete: keep it then move on to next rune
					continue
				}

				if currentRunePosition-match.start == len(match.body) {
					// unmatched: skip
					continue
				}

				currentMatchRune := match.body[currentRunePosition-match.start]
				if currentMatchRune != currentRune {
					// failed match, move on to next rune
					continue
				}

				// try to complete ongoing match
				if currentRunePosition-match.start == len(match.body)-1 {
					// we are close; the next step is to check the symbol ahead
					// if it is a lowercase letter, then it is not the end of match
					// but the beginning of the next word.
					//
					// NOTE(fredbi): this heuristic sometimes leads to counterintuitive splits and
					// perhaps (not sure yet) we should check against case _alternance_.
					//
					// Example:
					//
					// In the current version, in the sentence "IDS initialism", "ID" is recognized as an initialism,
					// leading to a split like "id_s_initialism" (or IDSInitialism),
					// whereas in the sentence "IDx initialism", it is not and produces something like
					// "i_d_x_initialism" (or IDxInitialism). The generated file name is not great.
					//
					// Both go identifiers are tolerated by linters.
					//
					// Notice that the slightly different input "IDs initialism" is correctly detected
					// as a pluralized initialism and produces something like "ids_initialism" (or IDsInitialism).

					if currentRunePosition < len(nameRunes)-1 {
						nextRune := nameRunes[currentRunePosition+1]

						// recognize a plural form for this initialism (only simple pluralization is supported)
						if nextRune == 's' && match.hasPlural == simplePlural {
							// detected a pluralized initialism
							match.body = append(match.body, nextRune)
							currentRunePosition++
							if currentRunePosition < len(nameRunes)-1 {
								nextRune = nameRunes[currentRunePosition+1]
								if newWord := unicode.IsLower(nextRune); newWord {
									// it is the start of a new word.
									// Match is only partial and the initialism is not recognized : move on
									continue
								}
							}

							// this is a pluralized match: keep it
							match.complete = true
							match.hasPlural = simplePlural
							match.end = currentRunePosition
							*newMatches = append(*newMatches, match)

							// match is complete: keep it then move on to next rune
							continue
						}

						if newWord := unicode.IsLower(nextRune); newWord {
							// it is the start of a new word
							// Match is only partial and the initialism is not recognized : move on
							continue
						}
					}

					match.complete = true
					match.end = currentRunePosition
				}

				// append the ongoing matching attempt (not necessarily complete)
				*newMatches = append(*newMatches, match)
			}
		}

		// check for new initialism matches, based on the first character
		for i, r := range s.initialismsRunes {
			if r[0] == currentRune {
				*newMatches = append(*newMatches, initialismMatch{
					start:     currentRunePosition,
					body:      r,
					complete:  false,
					hasPlural: s.initialismsPluralForm[i],
				})
			}
		}

		if matches != nil {
			poolOfMatches.RedeemMatches(matches)
		}
		matches = newMatches
	}

	// up to the caller to redeem this last slice
	return matches
}

func (s splitter) mapMatchesToNameLexems(nameRunes []rune, matches *initialismMatches) *[]nameLexem {
	nameLexems := poolOfLexems.BorrowLexems()

	var lastAcceptedMatch initialismMatch
	for _, match := range *matches {
		if !match.complete {
			continue
		}

		if firstMatch := lastAcceptedMatch.isZero(); firstMatch {
			s.appendBrokenDownCasualString(nameLexems, nameRunes[:match.start])
			*nameLexems = append(*nameLexems, s.breakInitialism(string(match.body)))

			lastAcceptedMatch = match

			continue
		}

		if overlappedMatch := match.start <= lastAcceptedMatch.end; overlappedMatch {
			continue
		}

		middle := nameRunes[lastAcceptedMatch.end+1 : match.start]
		s.appendBrokenDownCasualString(nameLexems, middle)
		*nameLexems = append(*nameLexems, s.breakInitialism(string(match.body)))

		lastAcceptedMatch = match
	}

	// we have not found any accepted matches
	if lastAcceptedMatch.isZero() {
		*nameLexems = (*nameLexems)[:0]
		s.appendBrokenDownCasualString(nameLexems, nameRunes)
	} else if lastAcceptedMatch.end+1 != len(nameRunes) {
		rest := nameRunes[lastAcceptedMatch.end+1:]
		s.appendBrokenDownCasualString(nameLexems, rest)
	}

	poolOfMatches.RedeemMatches(matches)

	return nameLexems
}

func (s splitter) breakInitialism(original string) nameLexem {
	return newInitialismNameLexem(original, original)
}

func (s splitter) appendBrokenDownCasualString(segments *[]nameLexem, str []rune) {
	currentSegment := poolOfBuffers.BorrowBuffer(len(str)) // unlike strings.Builder, bytes.Buffer initial storage can reused
	defer func() {
		poolOfBuffers.RedeemBuffer(currentSegment)
	}()

	addCasualNameLexem := func(original string) {
		*segments = append(*segments, newCasualNameLexem(original))
	}

	addInitialismNameLexem := func(original, match string) {
		*segments = append(*segments, newInitialismNameLexem(original, match))
	}

	var addNameLexem func(string)
	if s.postSplitInitialismCheck {
		addNameLexem = func(original string) {
			for i := range s.initialisms {
				if isEqualFoldIgnoreSpace(s.initialismsUpperCased[i], original) {
					addInitialismNameLexem(original, s.initialisms[i])

					return
				}
			}

			addCasualNameLexem(original)
		}
	} else {
		addNameLexem = addCasualNameLexem
	}

	// NOTE: (performance). The few remaining non-amortized allocations
	// lay in the code below: using String() forces
	for _, rn := range str {
		if replace, found := s.replaceFunc(rn); found {
			if currentSegment.Len() > 0 {
				addNameLexem(currentSegment.String())
				currentSegment.Reset()
			}

			if replace != "" {
				addNameLexem(replace)
			}

			continue
		}

		if !unicode.In(rn, unicode.L, unicode.M, unicode.N, unicode.Pc) {
			if currentSegment.Len() > 0 {
				addNameLexem(currentSegment.String())
				currentSegment.Reset()
			}

			continue
		}

		if unicode.IsUpper(rn) {
			if currentSegment.Len() > 0 {
				addNameLexem(currentSegment.String())
			}
			currentSegment.Reset()
		}

		currentSegment.WriteRune(rn)
	}

	if currentSegment.Len() > 0 {
		addNameLexem(currentSegment.String())
	}
}
