package reference

var (
	defaultTag = "latest"
)

// EnsureTagged adds the default tag "latest" to a reference if it only has
// a repo name.
func EnsureTagged(ref Named) NamedTagged {
	namedTagged, ok := ref.(NamedTagged)
	if !ok {
		namedTagged, err := WithTag(ref, defaultTag)
		if err != nil {
			// Default tag must be valid, to create a NamedTagged
			// type with non-validated input the WithTag function
			// should be used instead
			panic(err)
		}
		return namedTagged
	}
	return namedTagged
}
