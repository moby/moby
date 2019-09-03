//+build !windows

package jsonfilelog

func isSharingViolation(_ error) bool {
	return false
}
