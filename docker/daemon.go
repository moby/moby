// +build daemon

package main

import (
	"log"
	"net"

	"github.com/docker/docker/builtins"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/engine"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/sysinit"
)

const CanDaemon = true

func mainSysinit() {
	// Running in init mode
	sysinit.SysInit()
}

func mainDaemon() {
	if flag.NArg() != 0 {
		flag.Usage()
		return
	}

	if *bridgeName != "" && *bridgeIp != "" {
		log.Fatal("You specified -b & --bip, mutually exclusive options. Please specify only one.")
	}

	if !*flEnableIptables && !*flInterContainerComm {
		log.Fatal("You specified --iptables=false with --icc=false. ICC uses iptables to function. Please set --icc or --iptables to true.")
	}

	if net.ParseIP(*flDefaultIp) == nil {
		log.Fatalf("Specified --ip=%s is not in correct format \"0.0.0.0\".", *flDefaultIp)
	}

	eng := engine.New()
	signal.Trap(eng.Shutdown)
	// Load builtins
	if err := builtins.Register(eng); err != nil {
		log.Fatal(err)
	}

	// load the daemon in the background so we can immediately start
	// the http api so that connections don't fail while the daemon
	// is booting
	go func() {
		// Load plugin: httpapi
		job := eng.Job("initserver")
		// include the variable here too, for the server config
		job.Setenv("Pidfile", *pidfile)
		job.Setenv("Root", *flRoot)
		job.SetenvBool("AutoRestart", *flAutoRestart)
		job.SetenvList("Dns", flDns.GetAll())
		job.SetenvList("DnsSearch", flDnsSearch.GetAll())
		job.SetenvBool("EnableIptables", *flEnableIptables)
		job.SetenvBool("EnableIpForward", *flEnableIpForward)
		job.Setenv("BridgeIface", *bridgeName)
		job.Setenv("BridgeIP", *bridgeIp)
		job.Setenv("DefaultIp", *flDefaultIp)
		job.SetenvBool("InterContainerCommunication", *flInterContainerComm)
		job.Setenv("GraphDriver", *flGraphDriver)
		job.SetenvList("GraphOptions", flGraphOpts.GetAll())
		job.Setenv("ExecDriver", *flExecDriver)
		job.SetenvInt("Mtu", *flMtu)
		job.SetenvBool("EnableSelinuxSupport", *flSelinuxEnabled)
		job.SetenvList("Sockets", flHosts.GetAll())
		if err := job.Run(); err != nil {
			log.Fatal(err)
		}
		// after the daemon is done setting up we can tell the api to start
		// accepting connections
		if err := eng.Job("acceptconnections").Run(); err != nil {
			log.Fatal(err)
		}
	}()

	// TODO actually have a resolved graphdriver to show?
	log.Printf("docker daemon: %s %s; execdriver: %s; graphdriver: %s",
		dockerversion.VERSION,
		dockerversion.GITCOMMIT,
		*flExecDriver,
		*flGraphDriver)

	// Serve api
	job := eng.Job("serveapi", flHosts.GetAll()...)
	job.SetenvBool("Logging", true)
	job.SetenvBool("EnableCors", *flEnableCors)
	job.Setenv("Version", dockerversion.VERSION)
	job.Setenv("SocketGroup", *flSocketGroup)

	job.SetenvBool("Tls", *flTls)
	job.SetenvBool("TlsVerify", *flTlsVerify)
	job.Setenv("TlsCa", *flCa)
	job.Setenv("TlsCert", *flCert)
	job.Setenv("TlsKey", *flKey)
	job.SetenvBool("BufferRequests", true)
	if err := job.Run(); err != nil {
		log.Fatal(err)
	}
}
