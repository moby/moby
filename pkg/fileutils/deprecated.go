package fileutils

import "github.com/moby/patternmatcher"

type (
	// PatternMatcher allows checking paths against a list of patterns.
	//
	// Deprecated: use github.com/moby/patternmatcher.PatternMatcher
	PatternMatcher = patternmatcher.PatternMatcher

	// MatchInfo tracks information about parent dir matches while traversing a
	// filesystem.
	//
	// Deprecated: use github.com/moby/patternmatcher.MatchInfo
	MatchInfo = patternmatcher.MatchInfo

	// Pattern defines a single regexp used to filter file paths.
	//
	// Deprecated: use github.com/moby/patternmatcher.Pattern
	Pattern = patternmatcher.Pattern
)

var (
	// NewPatternMatcher creates a new matcher object for specific patterns that can
	// be used later to match against patterns against paths
	//
	// Deprecated: use github.com/moby/patternmatcher.New
	NewPatternMatcher = patternmatcher.New

	// Matches returns true if file matches any of the patterns
	// and isn't excluded by any of the subsequent patterns.
	//
	// This implementation is buggy (it only checks a single parent dir against the
	// pattern) and will be removed soon. Use MatchesOrParentMatches instead.
	//
	// Deprecated: use github.com/moby/patternmatcher.Matches
	Matches = patternmatcher.Matches

	// MatchesOrParentMatches returns true if file matches any of the patterns
	// and isn't excluded by any of the subsequent patterns.
	//
	// Deprecated: use github.com/moby/patternmatcher.MatchesOrParentMatches
	MatchesOrParentMatches = patternmatcher.MatchesOrParentMatches
)
