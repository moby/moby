package main

import (
	"flag"
	"fmt"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/utils"
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
)

func main() {
	if utils.SelfPath() == "/sbin/init" {
		// Running in init mode
		docker.SysInit()
		return
	}
	// FIXME: Switch d and D ? (to be more sshd like)
	flDaemon := flag.Bool("d", false, "Daemon mode")
	flDebug := flag.Bool("D", false, "Debug mode")
	flAutoRestart := flag.Bool("r", false, "Restart previously running containers")
	bridgeName := flag.String("b", "", "Attach containers to a pre-existing network bridge")
	pidfile := flag.String("p", "/var/run/docker.pid", "File containing process PID")
	flHost := flag.String("H", fmt.Sprintf("%s:%d", docker.DEFAULTHTTPHOST, docker.DEFAULTHTTPPORT), "Host:port to bind/connect to")
	flEnableCors := flag.Bool("api-enable-cors", false, "Enable CORS requests in the remote api.")
	flDns := flag.String("dns", "", "Set custom dns servers")
	flag.Parse()
	if *bridgeName != "" {
		docker.NetworkBridgeIface = *bridgeName
	} else {
		docker.NetworkBridgeIface = docker.DefaultNetworkBridge
	}

	protoAddr := parseHost(*flHost)
	protoAddrs := []string{protoAddr}

	if *flDebug {
		os.Setenv("DEBUG", "1")
	}
	docker.GITCOMMIT = GITCOMMIT
	if *flDaemon {
		if flag.NArg() != 0 {
			flag.Usage()
			return
		}
		if err := daemon(*pidfile, protoAddrs, *flAutoRestart, *flEnableCors, *flDns); err != nil {
			log.Fatal(err)
			os.Exit(-1)
		}
	} else {
		protoAddrParts := strings.SplitN(protoAddrs[0], "://", 2)
		if err := docker.ParseCommands(protoAddrParts[0], protoAddrParts[1], flag.Args()...); err != nil {
			log.Fatal(err)
			os.Exit(-1)
		}
	}
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

func daemon(pidfile string, protoAddrs []string, autoRestart, enableCors bool, flDns string) error {
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
	var dns []string
	if flDns != "" {
		dns = []string{flDns}
	}
	server, err := docker.NewServer(autoRestart, enableCors, dns)
	if err != nil {
		return err
	}
	chErrors := make(chan error, len(protoAddrs))
	for _, protoAddr := range protoAddrs {
		protoAddrParts := strings.SplitN(protoAddr, "://", 2)
		if protoAddrParts[0] == "unix" {
			syscall.Unlink(protoAddrParts[1]);
		} else if protoAddrParts[0] == "tcp" {
			if !strings.HasPrefix(protoAddrParts[1], "127.0.0.1") {
				log.Println("/!\\ DON'T BIND ON ANOTHER IP ADDRESS THAN 127.0.0.1 IF YOU DON'T KNOW WHAT YOU'RE DOING /!\\")
			}
		} else {
			log.Fatal("Invalid protocol format.")
			os.Exit(-1)
		}
		go func() {
			chErrors <- docker.ListenAndServe(protoAddrParts[0], protoAddrParts[1], server, true)
		}()
	}
	for i :=0 ; i < len(protoAddrs); i+=1 {
		err := <-chErrors
		if err != nil {
			return err
		}
	}
	return nil
}

func parseHost(addr string) string {
	if strings.HasPrefix(addr, "unix://") {
		return addr
	}
	host :=	docker.DEFAULTHTTPHOST
	port := docker.DEFAULTHTTPPORT
	if strings.HasPrefix(addr, "tcp://") {
		addr = strings.TrimPrefix(addr, "tcp://")
	}
	if strings.Contains(addr, ":") {
		hostParts := strings.Split(addr, ":")
		if len(hostParts) != 2 {
			log.Fatal("Invalid bind address format.")
			os.Exit(-1)
		}
		if hostParts[0] != "" {
			host = hostParts[0]
		}
		if p, err := strconv.Atoi(hostParts[1]); err == nil {		
			port = p
		}
	} else {
		host = addr
	}
	return fmt.Sprintf("tcp://%s:%d", host, port)
}
