package daemon

import (
    "fmt"
	"github.com/dotcloud/docker/server"
    "io/ioutil"
    "log"
    "os"
	"os/signal"
    "strconv"
    "strings"
    "syscall"
)

func Daemon(pidfile string, flGraphPath string, protoAddrs []string, autoRestart, enableCors bool, flDns string) error {
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
	srv, err := server.NewServer(flGraphPath, autoRestart, enableCors, dns)
	if err != nil {
		return err
	}
	chErrors := make(chan error, len(protoAddrs))
	for _, protoAddr := range protoAddrs {
		protoAddrParts := strings.SplitN(protoAddr, "://", 2)
		if protoAddrParts[0] == "unix" {
			syscall.Unlink(protoAddrParts[1])
		} else if protoAddrParts[0] == "tcp" {
			if !strings.HasPrefix(protoAddrParts[1], "127.0.0.1") {
				log.Println("/!\\ DON'T BIND ON ANOTHER IP ADDRESS THAN 127.0.0.1 IF YOU DON'T KNOW WHAT YOU'RE DOING /!\\")
			}
		} else {
			log.Fatal("Invalid protocol format.")
			os.Exit(-1)
		}
		go func() {
			chErrors <- server.ListenAndServe(protoAddrParts[0], protoAddrParts[1], srv, true)
		}()
	}
	for i := 0; i < len(protoAddrs); i += 1 {
		err := <-chErrors
		if err != nil {
			return err
		}
	}
	return nil
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

