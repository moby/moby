package reference

import "github.com/distribution/reference"

// Sort sorts string references preferring higher information references.
//
// Deprecated: use [reference.Sort].
func Sort(references []string) []string {
	return reference.Sort(references)
}
