package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dotcloud/docker/api"
	"github.com/dotcloud/docker/builtins"
	"github.com/dotcloud/docker/dockerversion"
	"github.com/dotcloud/docker/engine"
	flag "github.com/dotcloud/docker/pkg/mflag"
	"github.com/dotcloud/docker/pkg/opts"
	"github.com/dotcloud/docker/sysinit"
	"github.com/dotcloud/docker/utils"
)

func main() {
	if selfPath := utils.SelfPath(); selfPath == "/sbin/init" || selfPath == "/.dockerinit" {
		// Running in init mode
		sysinit.SysInit()
		return
	}

	var (
		flVersion            = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
		flDaemon             = flag.Bool([]string{"d", "-daemon"}, false, "Enable daemon mode")
		flDebug              = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
		flAutoRestart        = flag.Bool([]string{"r", "-restart"}, true, "Restart previously running containers")
		bridgeName           = flag.String([]string{"b", "-bridge"}, "", "Attach containers to a pre-existing network bridge; use 'none' to disable container networking")
		bridgeIp             = flag.String([]string{"#bip", "-bip"}, "", "Use this CIDR notation address for the network bridge's IP, not compatible with -b")
		pidfile              = flag.String([]string{"p", "-pidfile"}, "/var/run/docker.pid", "Path to use for daemon PID file")
		flRoot               = flag.String([]string{"g", "-graph"}, "/var/lib/docker", "Path to use as the root of the docker runtime")
		flEnableCors         = flag.Bool([]string{"#api-enable-cors", "-api-enable-cors"}, false, "Enable CORS headers in the remote API")
		flDns                = opts.NewListOpts(opts.ValidateIp4Address)
		flEnableIptables     = flag.Bool([]string{"#iptables", "-iptables"}, true, "Disable docker's addition of iptables rules")
		flEnableIpForward    = flag.Bool([]string{"#ip-forward", "-ip-forward"}, true, "Disable enabling of net.ipv4.ip_forward")
		flDefaultIp          = flag.String([]string{"#ip", "-ip"}, "0.0.0.0", "Default IP address to use when binding container ports")
		flInterContainerComm = flag.Bool([]string{"#icc", "-icc"}, true, "Enable inter-container communication")
		flGraphDriver        = flag.String([]string{"s", "-storage-driver"}, "", "Force the docker runtime to use a specific storage driver")
		flExecDriver         = flag.String([]string{"e", "-exec-driver"}, "", "Force the docker runtime to use a specific exec driver")
		flHosts              = opts.NewListOpts(api.ValidateHost)
		flMtu                = flag.Int([]string{"#mtu", "-mtu"}, 0, "Set the containers network MTU; if no value is provided: default to the default route MTU or 1500 if no default route is available")
	)
	flag.Var(&flDns, []string{"#dns", "-dns"}, "Force docker to use specific DNS servers")
	flag.Var(&flHosts, []string{"H", "-host"}, "tcp://host:port, unix://path/to/socket, fd://* or fd://socketfd to use in daemon mode. Multiple sockets can be specified")

	flag.Parse()

	if *flVersion {
		showVersion()
		return
	}
	if flHosts.Len() == 0 {
		defaultHost := os.Getenv("DOCKER_HOST")

		if defaultHost == "" || *flDaemon {
			// If we do not have a host, default to unix socket
			defaultHost = fmt.Sprintf("unix://%s", api.DEFAULTUNIXSOCKET)
		}
		if _, err := api.ValidateHost(defaultHost); err != nil {
			log.Fatal(err)
		}
		flHosts.Set(defaultHost)
	}

	if *bridgeName != "" && *bridgeIp != "" {
		log.Fatal("You specified -b & --bip, mutually exclusive options. Please specify only one.")
	}

	if *flDebug {
		os.Setenv("DEBUG", "1")
	}
	if *flDaemon {
		if flag.NArg() != 0 {
			flag.Usage()
			return
		}

		// set up the TempDir to use a canonical path
		tmp := os.TempDir()
		realTmp, err := utils.ReadSymlinkedDirectory(tmp)
		if err != nil {
			log.Fatalf("Unable to get the full path to the TempDir (%s): %s", tmp, err)
		}
		os.Setenv("TMPDIR", realTmp)

		// get the canonical path to the Docker root directory
		root := *flRoot
		var realRoot string
		if _, err := os.Stat(root); err != nil && os.IsNotExist(err) {
			realRoot = root
		} else {
			realRoot, err = utils.ReadSymlinkedDirectory(root)
			if err != nil {
				log.Fatalf("Unable to get the full path to root (%s): %s", root, err)
			}
		}

		eng, err := engine.New(realRoot)
		if err != nil {
			log.Fatal(err)
		}
		// Load builtins
		builtins.Register(eng)
		// load the daemon in the background so we can immediately start
		// the http api so that connections don't fail while the daemon
		// is booting
		go func() {
			// Load plugin: httpapi
			job := eng.Job("initserver")
			job.Setenv("Pidfile", *pidfile)
			job.Setenv("Root", realRoot)
			job.SetenvBool("AutoRestart", *flAutoRestart)
			job.SetenvList("Dns", flDns.GetAll())
			job.SetenvBool("EnableIptables", *flEnableIptables)
			job.SetenvBool("EnableIpForward", *flEnableIpForward)
			job.Setenv("BridgeIface", *bridgeName)
			job.Setenv("BridgeIP", *bridgeIp)
			job.Setenv("DefaultIp", *flDefaultIp)
			job.SetenvBool("InterContainerCommunication", *flInterContainerComm)
			job.Setenv("GraphDriver", *flGraphDriver)
			job.Setenv("ExecDriver", *flExecDriver)
			job.SetenvInt("Mtu", *flMtu)
			if err := job.Run(); err != nil {
				log.Fatal(err)
			}
			// after the daemon is done setting up we can tell the api to start
			// accepting connections
			if err := eng.Job("acceptconnections").Run(); err != nil {
				log.Fatal(err)
			}
		}()

		// Serve api
		job := eng.Job("serveapi", flHosts.GetAll()...)
		job.SetenvBool("Logging", true)
		job.SetenvBool("EnableCors", *flEnableCors)
		job.Setenv("Version", dockerversion.VERSION)
		if err := job.Run(); err != nil {
			log.Fatal(err)
		}
	} else {
		if flHosts.Len() > 1 {
			log.Fatal("Please specify only one -H")
		}
		protoAddrParts := strings.SplitN(flHosts.GetAll()[0], "://", 2)
		if err := api.ParseCommands(protoAddrParts[0], protoAddrParts[1], flag.Args()...); err != nil {
			if sterr, ok := err.(*utils.StatusError); ok {
				if sterr.Status != "" {
					log.Println(sterr.Status)
				}
				os.Exit(sterr.StatusCode)
			}
			log.Fatal(err)
		}
	}
}

func showVersion() {
	fmt.Printf("Docker version %s, build %s\n", dockerversion.VERSION, dockerversion.GITCOMMIT)
}
