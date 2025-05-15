//go:build !windows

package command

import (
	"github.com/spf13/pflag"
)

func installServiceFlags(flags *pflag.FlagSet) {
}
