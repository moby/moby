//go:build !windows

package main

// getLongPathName converts Windows short pathnames to full pathnames.
// For example C:\Users\ADMIN~1 --> C:\Users\Administrator.
// It is a no-op on non-Windows platforms
func getLongPathName(path string) (string, error) {
	return path, nil
}
