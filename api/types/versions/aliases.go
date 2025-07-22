package versions

import "github.com/moby/moby/api/types/versions"

// LessThan checks if a version is less than another
func LessThan(v, other string) bool {
	return versions.LessThan(v, other)
}

// LessThanOrEqualTo checks if a version is less than or equal to another
func LessThanOrEqualTo(v, other string) bool {
	return versions.LessThanOrEqualTo(v, other)
}

// GreaterThan checks if a version is greater than another
func GreaterThan(v, other string) bool {
	return versions.GreaterThan(v, other)
}

// GreaterThanOrEqualTo checks if a version is greater than or equal to another
func GreaterThanOrEqualTo(v, other string) bool {
	return versions.GreaterThanOrEqualTo(v, other)
}

// Equal checks if a version is equal to another
func Equal(v, other string) bool {
	return versions.Equal(v, other)
}
