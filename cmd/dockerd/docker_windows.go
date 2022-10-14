package main

import (
	"io"
	"path/filepath"

	"github.com/Microsoft/go-winio/pkg/etwlogrus"
	"github.com/sirupsen/logrus"
)

func runDaemon(opts *daemonOptions) error {
	daemonCli := NewDaemonCli()

	// On Windows, this may be launching as a service or with an option to
	// register the service.
	stop, runAsService, err := initService(daemonCli)
	if err != nil {
		return err
	}

	if stop {
		return nil
	}

	// Windows specific settings as these are not defaulted.
	if opts.configFile == "" {
		configDir, err := getDaemonConfDir(opts.daemonConfig.Root)
		if err != nil {
			return err
		}
		opts.configFile = filepath.Join(configDir, "daemon.json")
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

func initLogging(stdout, _ io.Writer) {
	// Maybe there is a historic reason why on non-Windows, stderr is used
	// for output. However, on Windows it makes no sense and there is no need.
	logrus.SetOutput(stdout)
	name := "docker" // FIXME(thaJeztah): what should the default be? "docker" or "Moby"?
	if flServiceName != nil && *flServiceName != "" {
		name = *flServiceName
	}

	// FIXME(thaJeztah): update this ID (it's calculated from the name, which was hard-coded to "Moby" (looks like it's case-insensitive for the GUID)
	// Provider ID: {6996f090-c5de-5082-a81e-5841acc3a635}
	// Hook isn't closed explicitly, as it will exist until process exit.
	// GUID is generated based on name - see Microsoft/go-winio/tools/etw-provider-gen.
	if hook, err := etwlogrus.NewHook(name); err == nil {
		logrus.AddHook(hook)
	}
	return
}
