package main

import (
	"encoding/json"
	"errors"
	"flag"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/nsinit"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
)

var (
	console string
	pipeFd  int
	logFile string
)

var (
	ErrUnsupported    = errors.New("Unsupported method")
	ErrWrongArguments = errors.New("Wrong argument count")
)

func init() {
	flag.StringVar(&console, "console", "", "console (pty slave) path")
	flag.StringVar(&logFile, "log", "none", "log options (none, stderr, or a file path)")
	flag.IntVar(&pipeFd, "pipe", 0, "sync pipe fd")

	flag.Parse()
}

func main() {
	if flag.NArg() < 1 {
		log.Fatal(ErrWrongArguments)
	}
	container, err := loadContainer()
	if err != nil {
		log.Fatal(err)
	}
	if err := setupLogging(); err != nil {
		log.Fatal(err)
	}
	switch flag.Arg(0) {
	case "exec": // this is executed outside of the namespace in the cwd
		log.SetPrefix("[nsinit exec] ")

		var exitCode int
		nspid, err := readPid()
		if err != nil {
			if !os.IsNotExist(err) {
				log.Fatal(err)
			}
		}
		if nspid > 0 {
			exitCode, err = nsinit.ExecIn(container, nspid, flag.Args()[1:])
		} else {
			exitCode, err = nsinit.Exec(container,
				os.Stdin, os.Stdout, os.Stderr, nil,
				logFile, flag.Args()[1:])
		}
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(exitCode)
	case "init": // this is executed inside of the namespace to setup the container
		log.SetPrefix("[nsinit init] ")
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		if flag.NArg() < 2 {
			log.Fatal(ErrWrongArguments)
		}
		syncPipe, err := nsinit.NewSyncPipeFromFd(0, uintptr(pipeFd))
		if err != nil {
			log.Fatal(err)
		}
		if err := nsinit.Init(container, cwd, console, syncPipe, flag.Args()[1:]); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("command not supported for nsinit %s", flag.Arg(0))
	}
}

func loadContainer() (*libcontainer.Container, error) {
	f, err := os.Open("container.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var container *libcontainer.Container
	if err := json.NewDecoder(f).Decode(&container); err != nil {
		return nil, err
	}
	return container, nil
}

func readPid() (int, error) {
	data, err := ioutil.ReadFile(".nspid")
	if err != nil {
		return -1, err
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return -1, err
	}
	return pid, nil
}

func setupLogging() (err error) {
	var writer io.Writer
	switch logFile {
	case "stderr":
		writer = os.Stderr
	case "none", "":
		writer = ioutil.Discard
	default:
		writer, err = os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0755)
		if err != nil {
			return err
		}
	}
	log.SetOutput(writer)
	return nil
}
