package strslice

import "github.com/moby/moby/api/types/strslice"

// StrSlice represents a string or an array of strings.
// We need to override the json decoder to accept both options.
type StrSlice = strslice.StrSlice
