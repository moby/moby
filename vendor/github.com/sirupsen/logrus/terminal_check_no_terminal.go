//go:build js || nacl || plan9 || wasi || wasip1

package logrus

func checkIfTerminal(_ any) bool {
	return false
}
