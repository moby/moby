//go:build tinygo

package logrus

func checkIfTerminal(_ any) bool {
	return false
}
