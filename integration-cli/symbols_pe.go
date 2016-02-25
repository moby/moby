//+build windows

package main

import (
	"debug/pe"
	"fmt"
)

func hasUnresolvedSymbol(dockerBinary, symbol string) (bool, error) {
	binary, err := pe.Open(dockerBinary)
	if err != nil {
		return false, err
	}
	defer binary.Close()

	if binary.Symbols == nil {
		return false, fmt.Errorf("unable to read symbol table from '%s'", dockerBinary)
	}

	for _, sym := range binary.Symbols {
		if sym.Name == symbol {
			return true, nil
		}
	}
	return false, fmt.Errorf("no unresolved reference to '%s' in '%s'", symbol, dockerBinary)
}
