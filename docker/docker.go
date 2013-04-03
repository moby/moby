package main

import (
	"flag"
	"fmt"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/rcli"
	"github.com/dotcloud/docker/term"
	"io"
	"log"
	"os"
	"os/signal"
)

var GIT_COMMIT string

func main() {
	if docker.SelfPath() == "/sbin/init" {
		// Running in init mode
		docker.SysInit()
		return
	}
	// FIXME: Switch d and D ? (to be more sshd like)
	flDaemon := flag.Bool("d", false, "Daemon mode")
	flDebug := flag.Bool("D", false, "Debug mode")
	bridgeName := flag.String("b", "", "Attach containers to a pre-existing network bridge")
	flag.Parse()
	if *bridgeName != "" {
		docker.NetworkBridgeIface = *bridgeName
	} else {
		docker.NetworkBridgeIface = docker.DefaultNetworkBridge
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
		if err := daemon(); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := runCommand(flag.Args()); err != nil {
			log.Fatal(err)
		}
	}
}

func daemon() error {
	service, err := docker.NewServer()
	if err != nil {
		return err
	}
	return rcli.ListenAndServe("tcp", "127.0.0.1:4242", service)
}

func setRawTerminal() (*term.State, error) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		_ = <-c
		term.Restore(int(os.Stdin.Fd()), oldState)
		os.Exit(0)
	}()
	return oldState, err
}

func restoreTerminal(state *term.State) {
	term.Restore(int(os.Stdin.Fd()), state)
}

type DockerLocalConn struct {
	file       *os.File
	savedState *term.State
}

func newDockerLocalConn(output *os.File) *DockerLocalConn {
	return &DockerLocalConn{file: output}
}

func (c *DockerLocalConn) Read(b []byte) (int, error) { return c.file.Read(b) }

func (c *DockerLocalConn) Write(b []byte) (int, error) { return c.file.Write(b) }

func (c *DockerLocalConn) Close() error {
	if c.savedState != nil {
		restoreTerminal(c.savedState)
		c.savedState = nil
	}
	return c.file.Close()
}

func (c *DockerLocalConn) CloseWrite() error { return nil }

func (c *DockerLocalConn) CloseRead() error { return nil }

func (c *DockerLocalConn) GetOptions() *rcli.DockerConnOptions { return nil }

func (c *DockerLocalConn) SetOptionRawTerminal() {
	if state, err := setRawTerminal(); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Can't set the terminal in raw mode: %v",
			err.Error(),
		)
	} else {
		c.savedState = state
	}
}

func runCommand(args []string) error {
	// FIXME: we want to use unix sockets here, but net.UnixConn doesn't expose
	// CloseWrite(), which we need to cleanly signal that stdin is closed without
	// closing the connection.
	// See http://code.google.com/p/go/issues/detail?id=3345
	if conn, err := rcli.Call("tcp", "127.0.0.1:4242", args...); err == nil {
		options := conn.GetOptions()
		if options.RawTerminal &&
			term.IsTerminal(int(os.Stdin.Fd())) &&
			os.Getenv("NORAW") == "" {
			if oldState, err := setRawTerminal(); err != nil {
				return err
			} else {
				defer restoreTerminal(oldState)
			}
		}
		receiveStdout := docker.Go(func() error {
			_, err := io.Copy(os.Stdout, conn)
			return err
		})
		sendStdin := docker.Go(func() error {
			_, err := io.Copy(conn, os.Stdin)
			if err := conn.CloseWrite(); err != nil {
				log.Printf("Couldn't send EOF: " + err.Error())
			}
			return err
		})
		if err := <-receiveStdout; err != nil {
			return err
		}
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			if err := <-sendStdin; err != nil {
				return err
			}
		}
	} else {
		service, err := docker.NewServer()
		if err != nil {
			return err
		}
		dockerConn := newDockerLocalConn(os.Stdout)
		defer dockerConn.Close()
		if err := rcli.LocalCall(service, os.Stdin, dockerConn, args...); err != nil {
			return err
		}
	}
	return nil
}
