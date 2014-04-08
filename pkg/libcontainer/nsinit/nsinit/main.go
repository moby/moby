package main

import (
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/nsinit"
)

var (
	root, console, logs string
	pipeFd              int
)

func registerFlags() {
	flag.StringVar(&console, "console", "", "console (pty slave) path")
	flag.IntVar(&pipeFd, "pipe", 0, "sync pipe fd")
	flag.StringVar(&root, "root", ".", "root for storing configuration data")
	flag.StringVar(&logs, "log", "none", "set stderr or a filepath to enable logging")

	flag.Parse()
}

func main() {
	registerFlags()

	if flag.NArg() < 1 {
		log.Fatalf("wrong number of argments %d", flag.NArg())
	}
	container, err := loadContainer()
	if err != nil {
		log.Fatalf("Unable to load container: %s", err)
	}
	l, err := getLogger("[exec] ")
	if err != nil {
		log.Fatal(err)
	}

	ns, err := newNsInit(l)
	if err != nil {
		log.Fatalf("Unable to initialize nsinit: %s", err)
	}

	switch flag.Arg(0) {
	case "exec": // this is executed outside of the namespace in the cwd
		var exitCode int
		nspid, err := readPid()
		if err != nil {
			if !os.IsNotExist(err) {
				l.Fatalf("Unable to read pid: %s", err)
			}
		}
		if nspid > 0 {
			exitCode, err = ns.ExecIn(container, nspid, flag.Args()[1:])
		} else {
			term := nsinit.NewTerminal(os.Stdin, os.Stdout, os.Stderr, container.Tty)
			exitCode, err = ns.Exec(container, term, flag.Args()[1:])
		}
		if err != nil {
			l.Fatalf("Failed to exec: %s", err)
		}
		os.Exit(exitCode)
	case "init": // this is executed inside of the namespace to setup the container
		cwd, err := os.Getwd()
		if err != nil {
			l.Fatal(err)
		}
		if flag.NArg() < 2 {
			l.Fatalf("wrong number of argments %d", flag.NArg())
		}
		syncPipe, err := nsinit.NewSyncPipeFromFd(0, uintptr(pipeFd))
		if err != nil {
			l.Fatalf("Unable to create sync pipe: %s", err)
		}
		if err := ns.Init(container, cwd, console, syncPipe, flag.Args()[1:]); err != nil {
			l.Fatalf("Unable to initialize for container: %s", err)
		}
	default:
		l.Fatalf("command not supported for nsinit %s", flag.Arg(0))
	}
}

func loadContainer() (*libcontainer.Container, error) {
	f, err := os.Open(filepath.Join(root, "container.json"))
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
	data, err := ioutil.ReadFile(filepath.Join(root, "pid"))
	if err != nil {
		return -1, err
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return -1, err
	}
	return pid, nil
}

func newNsInit(l *log.Logger) (nsinit.NsInit, error) {
	return nsinit.NewNsInit(&nsinit.DefaultCommandFactory{root}, &nsinit.DefaultStateWriter{root}, l), nil
}

func getLogger(prefix string) (*log.Logger, error) {
	var w io.Writer
	switch logs {
	case "", "none":
		w = ioutil.Discard
	case "stderr":
		w = os.Stderr
	default: // we have a filepath
		f, err := os.OpenFile(logs, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0755)
		if err != nil {
			return nil, err
		}
		w = f
	}
	return log.New(w, prefix, log.LstdFlags), nil
}
