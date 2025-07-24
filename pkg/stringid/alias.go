// Package stringid provides helper functions for dealing with string identifiers.
package stringid

import "github.com/moby/moby/client/pkg/stringid"

// TruncateID returns a shorthand version of a string identifier for presentation.
//
// Deprecated: use [stringid.TruncateID]. This package will be removed in the next release.
func TruncateID(id string) string {
	return stringid.TruncateID(id)
}

// GenerateRandomID returns a unique, 64-character ID consisting of a-z, 0-9.
//
// Deprecated: use [stringid.GenerateRandomID]. This package will be removed in the next release.
func GenerateRandomID() string {
	return stringid.GenerateRandomID()
}
