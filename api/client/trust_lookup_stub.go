// +build !experimental

package client

// trust_should_lookup checks whether the trust system should be
// used to lookup a reference for an image. Returns true if trust
// is enabled and we are not already doing a pull by digest.
func trustShouldLookup(byDigest, official bool) bool {
	return isTrusted() && !byDigest
}
