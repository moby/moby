// go list ./... and go test ./... will not pick up this package without this
// file, because go ? ./... does not honor build tags.

// e.g. "go list -tags pkcs11 ./..." will not list this package if all the
// files in it have a build tag.

// See https://github.com/golang/go/issues/11246

package yubikey
