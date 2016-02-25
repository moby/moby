//+build !linux,!windows

package main

import (
	"fmt"
)

func hasUnresolvedSymbol(dockerBinary, symbol string) (bool, error) {
	return false, fmt.Errorf("don't know how to check '%s' for references to '%s'", dockerBinary, symbol)
}
