// +build experimental

package client

// trust_should_lookup checks whether the trust system should be used to
// lookup a reference for an image. In this experimental version, it returns
// true if trust is enabled and we are not already doing a pull by digest,
// and also returns true if it is an official image being that is not
// already being pulled by digest
func trustShouldLookup(byDigest, official bool) bool {
	return (isTrusted() || official) && !byDigest
}
