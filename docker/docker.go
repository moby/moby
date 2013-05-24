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
	GIT_COMMIT string
)

func main() {
	if utils.SelfPath() == "/sbin/init" {
		// Running in init mode
		docker.SysInit()
		return
	}
	host := "127.0.0.1"
	port := 4243
	// FIXME: Switch d and D ? (to be more sshd like)
	flDaemon := flag.Bool("d", false, "Daemon mode")
	flDebug := flag.Bool("D", false, "Debug mode")
	flAutoRestart := flag.Bool("r", false, "Restart previously running containers")
	bridgeName := flag.String("b", "", "Attach containers to a pre-existing network bridge")
	pidfile := flag.String("p", "/var/run/docker.pid", "File containing process PID")
	flHost := flag.String("H", fmt.Sprintf("%s:%d", host, port), "Host:port to bind/connect to")
	flag.Parse()
	if *bridgeName != "" {
		docker.NetworkBridgeIface = *bridgeName
	} else {
		docker.NetworkBridgeIface = docker.DefaultNetworkBridge
	}

	if strings.Contains(*flHost, ":") {
		hostParts := strings.Split(*flHost, ":")
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
		host = *flHost
	}

	if *flDebug {
		os.Setenv("DEBUG", "1")
	}
	docker.GIT_COMMIT = GIT_COMMIT
	if *flDaemon {
		if flag.NArg() != 0 {
			flag.Usage()
			return
		}
		if err := daemon(*pidfile, host, port, *flAutoRestart); err != nil {
			log.Fatal(err)
			os.Exit(-1)
		}
	} else {
		if err := docker.ParseCommands(host, port, flag.Args()...); err != nil {
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

func daemon(pidfile, addr string, port int, autoRestart bool) error {
	if addr != "127.0.0.1" {
		log.Println("/!\\ DON'T BIND ON ANOTHER IP ADDRESS THAN 127.0.0.1 IF YOU DON'T KNOW WHAT YOU'RE DOING /!\\")
	}
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

	server, err := docker.NewServer(autoRestart)
	if err != nil {
		return err
	}

	return docker.ListenAndServe(fmt.Sprintf("%s:%d", addr, port), server, true)
}
