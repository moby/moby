// +build !daemon

package main

import flag "github.com/docker/docker/pkg/mflag"

var cmd *flag.FlagSet

func installDaemonFlags() error {

	return nil

}

func parseDaemonFlags(cmd *flag.FlagSet, args ...string) error {

	return nil
}
