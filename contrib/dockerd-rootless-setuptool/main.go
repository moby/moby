package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	dockerdRootlessSh = "dockerd-rootless.sh"
	rootfulDockerSock = "/var/run/docker.sock"
)

var (
	// ignoreRootful is set by global flag --ignore-rootful.
	// Corresponds to $FORCE_ROOTLESS_INSTALL in https://get.docker.com/rootless .
	ignoreRootful bool
	// skipIptables is set by global flag --skip-iptables
	// Corresponds to $SKIP_IPTABLES in https://get.docker.com/rootless .
	skipIptables bool
)

func main() {
	rootCmd := &cobra.Command{
		Short: fmt.Sprintf("A setup tool for %s. Requires %s to be present in $PATH. See https://docs.docker.com/engine/security/rootless/ for the usage.", dockerdRootlessSh, dockerdRootlessSh),
	}
	rootCmd.PersistentFlags().BoolVar(&ignoreRootful, "ignore-rootful", false, fmt.Sprintf("ignore rootful Docker (%s)", rootfulDockerSock))
	rootCmd.PersistentFlags().BoolVar(&skipIptables, "skip-iptables", false, "ignore mising iptables")

	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Check prerequisites",
		Run:   runCheckCmd,
	}

	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup systemd (if available) and show how to run rootless Docker",
		Run:   runSetupCmd,
	}

	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.Execute()
}
