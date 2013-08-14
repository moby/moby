package main

import (
	"flag"
	"fmt"
	"github.com/dotcloud/docker/core"
	"github.com/dotcloud/docker/cli"
	"github.com/dotcloud/docker/daemon"
	"github.com/dotcloud/docker/server"
	"github.com/dotcloud/docker/utils"
	"log"
	"os"
	"strings"
)

var (
	GITCOMMIT string
)

func main() {
	if selfPath := utils.SelfPath(); selfPath == "/sbin/init" || selfPath == "/.dockerinit" {
		// Running in init mode
		core.SysInit()
		return
	}
	// FIXME: Switch d and D ? (to be more sshd like)
	flDaemon := flag.Bool("d", false, "Daemon mode")
	flDebug := flag.Bool("D", false, "Debug mode")
	flAutoRestart := flag.Bool("r", false, "Restart previously running containers")
	bridgeName := flag.String("b", "", "Attach containers to a pre-existing network bridge. Use 'none' to disable container networking")
	pidfile := flag.String("p", "/var/run/core.pid", "File containing process PID")
	flGraphPath := flag.String("g", "/var/lib/docker", "Path to graph storage base dir.")
	flEnableCors := flag.Bool("api-enable-cors", false, "Enable CORS requests in the remote api.")
	flDns := flag.String("dns", "", "Set custom dns servers")
	flHosts := core.ListOpts{fmt.Sprintf("unix://%s", server.DEFAULTUNIXSOCKET)}
	flag.Var(&flHosts, "H", "tcp://host:port to bind/connect to or unix://path/to/socket to use")
	flag.Parse()
	if len(flHosts) > 1 {
		flHosts = flHosts[1:] //trick to display a nice default value in the usage
	}
	for i, flHost := range flHosts {
		flHosts[i] = utils.ParseHost(server.DEFAULTHTTPHOST, server.DEFAULTHTTPPORT, flHost)
	}

	if *bridgeName != "" {
		core.NetworkBridgeIface = *bridgeName
	} else {
		core.NetworkBridgeIface = core.DefaultNetworkBridge
	}
	if *flDebug {
		os.Setenv("DEBUG", "1")
	}
	core.GITCOMMIT = GITCOMMIT
	if *flDaemon {
		if flag.NArg() != 0 {
			flag.Usage()
			return
		}
		if err := daemon.Daemon(*pidfile, *flGraphPath, flHosts, *flAutoRestart, *flEnableCors, *flDns); err != nil {
			log.Fatal(err)
			os.Exit(-1)
		}
	} else {
		if len(flHosts) > 1 {
			log.Fatal("Please specify only one -H")
			return
		}
		protoAddrParts := strings.SplitN(flHosts[0], "://", 2)
		if err := cli.ParseCommands(protoAddrParts[0], protoAddrParts[1], flag.Args()...); err != nil {
			log.Fatal(err)
			os.Exit(-1)
		}
	}
}
