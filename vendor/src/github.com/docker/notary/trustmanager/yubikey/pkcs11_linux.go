// +build pkcs11,linux

package yubikey

var possiblePkcs11Libs = []string{
	"/usr/lib/libykcs11.so",
	"/usr/lib/x86_64-linux-gnu/libykcs11.so",
	"/usr/local/lib/libykcs11.so",
}
