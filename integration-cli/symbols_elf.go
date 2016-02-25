//+build linux

package main

import (
	"debug/elf"
	"fmt"
)

func hasUnresolvedSymbol(dockerBinary, symbol string) (bool, error) {
	binary, err := elf.Open(dockerBinary)
	if err != nil {
		return false, err
	}
	defer binary.Close()

	symbols, err := binary.DynamicSymbols()
	if err != nil {
		return false, err
	}

	for _, sym := range symbols {
		if sym.Name == symbol {
			return true, nil
		}
	}
	return false, fmt.Errorf("no unresolved reference to '%s' in '%s'", symbol, dockerBinary)
}
