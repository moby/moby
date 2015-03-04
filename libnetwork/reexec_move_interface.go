package libnetwork

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"syscall"

	"github.com/vishvananda/netlink"
)

type setupError struct {
	Message string
}

func (s setupError) Error() string {
	return s.Message
}

func reexecMoveInterface() {
	runtime.LockOSThread()

	var (
		err  error
		pipe = os.NewFile(3, "child")
	)

	defer func() {
		if err != nil {
			ioutil.ReadAll(pipe)
			if err := json.NewEncoder(pipe).Encode(setupError{Message: err.Error()}); err != nil {
				panic(err)
			}
		}
		pipe.Close()
	}()

	n := &Interface{}
	if err = json.NewDecoder(pipe).Decode(n); err == nil {
		err = setupInNS(os.Args[1], n)
	}
}

func setupInNS(nsPath string, settings *Interface) error {
	f, err := os.OpenFile(nsPath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed get network namespace %q: %v", nsPath, err)
	}

	// Find the network inteerface identified by the SrcName attribute.
	iface, err := netlink.LinkByName(settings.SrcName)
	if err != nil {
		return err
	}

	// Move the network interface to the destination namespace.
	nsFD := f.Fd()
	if err := netlink.LinkSetNsFd(iface, int(nsFD)); err != nil {
		return err
	}
	f.Close()

	// Move the executing code to the destination namespace so we can start
	// configure the interface.
	if err := setns(nsFD, syscall.CLONE_NEWNET); err != nil {
		return err
	}

	// Configure the interface now this is moved in the proper namespace.
	if err := configureInterface(iface, settings); err != nil {
		return err
	}

	// Up the interface.
	return netlink.LinkSetUp(iface)
}
