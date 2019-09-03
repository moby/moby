package jsonfilelog

import (
	"os"

	"golang.org/x/sys/windows"
)

func isSharingViolation(err error) bool {
	switch err := err.(type) {
	case *os.PathError:
		if err.Err == windows.ERROR_SHARING_VIOLATION {
			return true
		}
	}
	return false
}
