//go:build !windows

package main

import (
	"github.com/spf13/pflag"
)

func installServiceFlags(_ *pflag.FlagSet) {
}
