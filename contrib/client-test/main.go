package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/client"
	"github.com/docker/docker/opts"
)

func main() {
	if err := xmain(); err != nil {
		logrus.Fatal(err)
		os.Exit(1)
	}
}

func xmain() error {
	host := client.DefaultDockerHost
	if len(os.Args) == 2 {
		host = os.Args[1]
	}
	if len(os.Args) > 2 {
		return fmt.Errorf("Usage: %s HOST", os.Args[0])
	}
	parsedHost, err := opts.ParseHost(false, host)
	if err != nil {
		return err
	}
	logrus.Infof("Parsed %q into %q", host, parsedHost)
	logrus.Infof("Connecting to the daemon %q, using version %q", parsedHost, api.DefaultVersion)
	cli, err := client.NewClient(parsedHost, api.DefaultVersion, nil, nil)
	if err != nil {
		return err
	}
	logrus.Infof("Connected to the daemon %q.", cli.DaemonHost())

	logrus.Infof("Daemon Info:")
	inf, err := cli.Info(context.Background())
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(inf, "", "  ")
	if err != nil {
		return err
	}
	os.Stdout.Write(b)

	return cli.Close()
}
