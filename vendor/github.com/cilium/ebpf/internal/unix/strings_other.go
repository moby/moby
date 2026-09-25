//go:build !linux && !windows

package unix

func BytePtrFromString(s string) (*byte, error) {
	return nil, errNonLinux()
}

func ByteSliceToString(s []byte) string {
	return ""
}

func ByteSliceFromString(s string) ([]byte, error) {
	return nil, errNonLinux()
}
