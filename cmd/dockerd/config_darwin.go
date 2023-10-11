package main

import (
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/opts"
	"github.com/spf13/pflag"
)

func configureCertsDir() {}

func installConfigFlags(conf *config.Config, flags *pflag.FlagSet) error {
	if err := installCommonConfigFlags(conf, flags); err != nil {
		return err
	}

	flags.Var(opts.NewNamedRuntimeOpt("runtimes", &conf.Runtimes, config.StockRuntimeName), "add-runtime", "Register an additional OCI compatible runtime")
	flags.StringVarP(&conf.SocketGroup, "group", "G", "docker", "Group for the unix socket")

	return nil
}
