package main

import (
	"flag"
	"fmt"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/sysinit"
	"github.com/dotcloud/docker/utils"
	"github.com/dotcloud/docker/engine"
	"io/ioutil"
	"log"
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
		sysinit.SysInit()
		return
	}
	// FIXME: Switch d and D ? (to be more sshd like)
	flVersion := flag.Bool("v", false, "Print version information and quit")
	flDaemon := flag.Bool("d", false, "Daemon mode")
	flDebug := flag.Bool("D", false, "Debug mode")
	flAutoRestart := flag.Bool("r", true, "Restart previously running containers")
	bridgeName := flag.String("b", "", "Attach containers to a pre-existing network bridge. Use 'none' to disable container networking")
	pidfile := flag.String("p", "/var/run/docker.pid", "File containing process PID")
	flRoot := flag.String("g", "/var/lib/docker", "Path to use as the root of the docker runtime.")
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
		eng, err := engine.New(*flRoot)
		if err != nil {
			log.Fatal(err)
		}
		job, err := eng.Job("serveapi")
		if err != nil {
			log.Fatal(err)
		}
		job.Setenv("Pidfile", *pidfile)
		job.Setenv("Root", *flRoot)
		job.SetenvBool("AutoRestart", *flAutoRestart)
		job.SetenvBool("EnableCors", *flEnableCors)
		job.Setenv("Dns", *flDns)
		job.SetenvBool("EnableIptables", *flEnableIptables)
		job.Setenv("BridgeIface", *bridgeName)
		job.SetenvList("ProtoAddresses", flHosts)
		job.Setenv("DefaultIp", *flDefaultIp)
		job.SetenvBool("InterContainerCommunication", *flInterContainerComm)
		if err := daemon(job, *pidfile); err != nil {
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

// daemon runs `job` as a daemon. 
// A pidfile is created for the duration of the job,
// and all signals are intercepted.
func daemon(job *engine.Job, pidfile string) error {
	if err := createPidFile(pidfile); err != nil {
		log.Fatal(err)
	}
	defer removePidFile(pidfile)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill, os.Signal(syscall.SIGTERM))
	go func() {
		sig := <-c
		log.Printf("Received signal '%v', exiting\n", sig)
		removePidFile(pidfile)
		os.Exit(0)
	}()
	return job.Run()
}
