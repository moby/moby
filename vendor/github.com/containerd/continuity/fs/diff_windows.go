package fs

import (
	"os"

	"golang.org/x/sys/windows"
)

func detectDirDiff(upper, lower string) *diffDirOptions {
	return nil
}

func compareSysStat(s1, s2 interface{}) (bool, error) {
	f1, ok := s1.(windows.Win32FileAttributeData)
	if !ok {
		return false, nil
	}
	f2, ok := s2.(windows.Win32FileAttributeData)
	if !ok {
		return false, nil
	}
	return f1.FileAttributes == f2.FileAttributes, nil
}

func compareCapabilities(p1, p2 string) (bool, error) {
	// TODO: Use windows equivalent
	return true, nil
}

func isLinked(os.FileInfo) bool {
	return false
}
