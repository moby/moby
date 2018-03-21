package main

import (
	"path/filepath"

	_ "github.com/docker/docker/autogen/winresources/dockerd"
	"github.com/sirupsen/logrus"
)

func runDaemon(opts *daemonOptions) error {
	daemonCli := NewDaemonCli()

	// On Windows, this may be launching as a service or with an option to
	// register the service.
	stop, runAsService, err := initService(daemonCli)
	if err != nil {
		logrus.Fatal(err)
	}

	if stop {
		return nil
	}

	// Windows specific settings as these are not defaulted.
	if opts.configFile == "" {
		opts.configFile = filepath.Join(opts.daemonConfig.Root, `config\daemon.json`)
	}
	if runAsService {
		// If Windows SCM manages the service - no need for PID files
		opts.daemonConfig.Pidfile = ""
	} else if opts.daemonConfig.Pidfile == "" {
		opts.daemonConfig.Pidfile = filepath.Join(opts.daemonConfig.Root, "docker.pid")
	}

	err = daemonCli.start(opts)
	notifyShutdown(err)
	return err
}
