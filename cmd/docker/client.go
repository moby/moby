package main

import (
	"path/filepath"

	"github.com/docker/docker/cli"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/docker/docker/cliconfig"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
)

var (
	commonFlags = cliflags.InitCommonFlags()
	clientFlags = &cli.ClientFlags{FlagSet: new(flag.FlagSet), Common: commonFlags}
)

func init() {

	client := clientFlags.FlagSet
	client.StringVar(&clientFlags.ConfigDir, []string{"-config"}, cliconfig.ConfigDir(), "Location of client config files")

	clientFlags.PostParse = func() {
		clientFlags.Common.PostParse()

		if clientFlags.ConfigDir != "" {
			cliconfig.SetConfigDir(clientFlags.ConfigDir)
		}

		if clientFlags.Common.TrustKey == "" {
			clientFlags.Common.TrustKey = filepath.Join(cliconfig.ConfigDir(), cliflags.DefaultTrustKeyFile)
		}

		if clientFlags.Common.Debug {
			utils.EnableDebug()
		}
	}
}
