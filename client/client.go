package client

import (
	"github.com/dotcloud/docker/future"
	"github.com/dotcloud/docker/rcli"
	"io"
	"log"
	"os"
)

// Run docker in "simple mode": run a single command and return.
func SimpleMode(args []string) error {
	var oldState *State
	var err error
	if IsTerminal(0) && os.Getenv("NORAW") == "" {
		oldState, err = MakeRaw(0)
		if err != nil {
			return err
		}
		defer Restore(0, oldState)
	}
	// FIXME: we want to use unix sockets here, but net.UnixConn doesn't expose
	// CloseWrite(), which we need to cleanly signal that stdin is closed without
	// closing the connection.
	// See http://code.google.com/p/go/issues/detail?id=3345
	conn, err := rcli.Call("tcp", "127.0.0.1:4242", args...)
	if err != nil {
		return err
	}
	receive_stdout := future.Go(func() error {
		_, err := io.Copy(os.Stdout, conn)
		return err
	})
	send_stdin := future.Go(func() error {
		_, err := io.Copy(conn, os.Stdin)
		if err := conn.CloseWrite(); err != nil {
			log.Printf("Couldn't send EOF: " + err.Error())
		}
		return err
	})
	if err := <-receive_stdout; err != nil {
		return err
	}
	if oldState != nil {
		Restore(0, oldState)
	}
	if !IsTerminal(0) {
		if err := <-send_stdin; err != nil {
			return err
		}
	}
	return nil
}
