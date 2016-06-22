// +build !windows

package main

import (
	"github.com/spf13/pflag"
)

func initService() (bool, error) {
	return false, nil
}

func installServiceFlags(flags *pflag.FlagSet) {
}
