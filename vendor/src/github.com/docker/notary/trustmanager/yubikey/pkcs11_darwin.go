// +build pkcs11,darwin

package yubikey

var possiblePkcs11Libs = []string{
	"/usr/local/lib/libykcs11.dylib",
	"/usr/local/docker/lib/libykcs11.dylib",
	"/usr/local/docker-experimental/lib/libykcs11.dylib",
}
