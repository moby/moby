package main

import (
	"flag"
	"fmt"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

var (
	GITCOMMIT string
	VERSION   string
)

func main() {
	if selfPath := utils.SelfPath(); selfPath == "/sbin/init" || selfPath == "/.dockerinit" {
		// Running in init mode
		docker.SysInit()
		return
	}
	// FIXME: Switch d and D ? (to be more sshd like)
	flVersion := flag.Bool("v", false, "Print version information and quit")
	flDaemon := flag.Bool("d", false, "Daemon mode")
	flDebug := flag.Bool("D", false, "Debug mode")
	flAutoRestart := flag.Bool("r", true, "Restart previously running containers")
	bridgeName := flag.String("b", "", "Attach containers to a pre-existing network bridge. Use 'none' to disable container networking")
	pidfile := flag.String("p", "/var/run/docker.pid", "File containing process PID")
	flGraphPath := flag.String("g", "/var/lib/docker", "Path to graph storage base dir.")
	flEnableCors := flag.Bool("api-enable-cors", false, "Enable CORS requests in the remote api.")
	flDns := flag.String("dns", "", "Set custom dns servers")
	flHosts := utils.ListOpts{fmt.Sprintf("unix://%s", docker.DEFAULTUNIXSOCKET)}
	flag.Var(&flHosts, "H", "tcp://host:port to bind/connect to or unix://path/to/socket to use")
	flEnableIptables := flag.Bool("iptables", true, "Disable iptables within docker")
	flDefaultIp := flag.String("ip", "0.0.0.0", "Default ip address to use when binding a containers ports")
	flInterContainerComm := flag.Bool("enable-container-comm", false, "Enable inter-container communication")

	flag.Parse()

	if *flVersion {
		showVersion()
		return
	}
	if len(flHosts) > 1 {
		flHosts = flHosts[1:] //trick to display a nice default value in the usage
	}
	for i, flHost := range flHosts {
		host, err := utils.ParseHost(docker.DEFAULTHTTPHOST, docker.DEFAULTHTTPPORT, flHost)
		if err == nil {
			flHosts[i] = host
		} else {
			log.Fatal(err)
		}
	}

	bridge := docker.DefaultNetworkBridge
	if *bridgeName != "" {
		bridge = *bridgeName
	}
	if *flDebug {
		os.Setenv("DEBUG", "1")
	}
	docker.GITCOMMIT = GITCOMMIT
	docker.VERSION = VERSION
	if *flDaemon {
		if flag.NArg() != 0 {
			flag.Usage()
			return
		}
		var dns []string
		if *flDns != "" {
			dns = []string{*flDns}
		}

		ip := net.ParseIP(*flDefaultIp)

		config := &docker.DaemonConfig{
			Pidfile:                     *pidfile,
			GraphPath:                   *flGraphPath,
			AutoRestart:                 *flAutoRestart,
			EnableCors:                  *flEnableCors,
			Dns:                         dns,
			EnableIptables:              *flEnableIptables,
			BridgeIface:                 bridge,
			ProtoAddresses:              flHosts,
			DefaultIp:                   ip,
			InterContainerCommunication: *flInterContainerComm,
		}
		if err := daemon(config); err != nil {
			log.Fatal(err)
		}
	} else {
		if len(flHosts) > 1 {
			log.Fatal("Please specify only one -H")
		}
		protoAddrParts := strings.SplitN(flHosts[0], "://", 2)
		if err := docker.ParseCommands(protoAddrParts[0], protoAddrParts[1], flag.Args()...); err != nil {
			if sterr, ok := err.(*utils.StatusError); ok {
				os.Exit(sterr.Status)
			}
			log.Fatal(err)
		}
	}
}

func showVersion() {
	fmt.Printf("Docker version %s, build %s\n", VERSION, GITCOMMIT)
}

func createPidFile(pidfile string) error {
	if pidString, err := ioutil.ReadFile(pidfile); err == nil {
		pid, err := strconv.Atoi(string(pidString))
		if err == nil {
			if _, err := os.Stat(fmt.Sprintf("/proc/%d/", pid)); err == nil {
				return fmt.Errorf("pid file found, ensure docker is not running or delete %s", pidfile)
			}
		}
	}

	file, err := os.Create(pidfile)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = fmt.Fprintf(file, "%d", os.Getpid())
	return err
}

func removePidFile(pidfile string) {
	if err := os.Remove(pidfile); err != nil {
		log.Printf("Error removing %s: %s", pidfile, err)
	}
}

func daemon(config *docker.DaemonConfig) error {
	if err := createPidFile(config.Pidfile); err != nil {
		log.Fatal(err)
	}
	defer removePidFile(config.Pidfile)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill, os.Signal(syscall.SIGTERM))
	go func() {
		sig := <-c
		log.Printf("Received signal '%v', exiting\n", sig)
		removePidFile(config.Pidfile)
		os.Exit(0)
	}()
	server, err := docker.NewServer(config)
	if err != nil {
		return err
	}
	chErrors := make(chan error, len(config.ProtoAddresses))
	for _, protoAddr := range config.ProtoAddresses {
		protoAddrParts := strings.SplitN(protoAddr, "://", 2)
		if protoAddrParts[0] == "unix" {
			syscall.Unlink(protoAddrParts[1])
		} else if protoAddrParts[0] == "tcp" {
			if !strings.HasPrefix(protoAddrParts[1], "127.0.0.1") {
				log.Println("/!\\ DON'T BIND ON ANOTHER IP ADDRESS THAN 127.0.0.1 IF YOU DON'T KNOW WHAT YOU'RE DOING /!\\")
			}
		} else {
			log.Fatal("Invalid protocol format.")
		}
		go func() {
			chErrors <- docker.ListenAndServe(protoAddrParts[0], protoAddrParts[1], server, true)
		}()
	}
	for i := 0; i < len(config.ProtoAddresses); i += 1 {
		err := <-chErrors
		if err != nil {
			return err
		}
	}
	return nil
}
