package ns

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/modprobe"
	"github.com/moby/moby/v2/daemon/libnetwork/nlwrap"
	"github.com/vishvananda/netns"
)

var (
	initNs   = netns.None()
	initNl   nlwrap.Handle
	initOnce sync.Once
	// NetlinkSocketsTimeout represents the default timeout duration for the sockets
	NetlinkSocketsTimeout = 3 * time.Second
)

// initHandles initializes a new network namespace
func initHandles() {
	var err error
	initNs, err = netns.Get()
	if err != nil {
		log.G(context.TODO()).Errorf("could not get initial namespace: %v", err)
	}

	initNl, err = nlwrap.NewHandle(getSupportedNlFamilies()...)
	if err != nil {
		// Fail fast to keep the invariant: NlHandle must be a valid handle
		panic(fmt.Sprintf("could not create netlink handle on initial (host) namespace: %v", err))
	}

	err = initNl.SetSocketTimeout(NetlinkSocketsTimeout)
	if err != nil {
		log.G(context.TODO()).Warnf("Failed to set the timeout on the default netlink handle sockets: %v", err)
	}
}

// ResetHandles resets the initial namespace and netlink handles.
// This is useful for testing to ensure a clean state. It will
// panic if called outside a test.
func ResetHandles() {
	if !testing.Testing() {
		panic("ResetHandles should only be called from tests")
	}
	if initNs.IsOpen() {
		initNs.Close()
		initNs = netns.None()
	}
	if initNl.Handle != nil {
		initNl.Close()
		initNl = nlwrap.Handle{}
	}
	initOnce = sync.Once{}
}

// ParseHandlerInt transforms the namespace handler into an integer
func ParseHandlerInt() int {
	return int(getHandler())
}

// GetHandler returns the namespace handler
func getHandler() netns.NsHandle {
	initOnce.Do(initHandles)
	return initNs
}

// NlHandle returns the netlink handler
func NlHandle() nlwrap.Handle {
	initOnce.Do(initHandles)
	return initNl
}

func getSupportedNlFamilies() []int {
	fams := []int{syscall.NETLINK_ROUTE}
	// NETLINK_XFRM test
	if err := checkXfrmSocket(); err != nil {
		log.G(context.TODO()).Warnf("Could not load necessary modules for IPSEC rules: %v", err)
	} else {
		fams = append(fams, syscall.NETLINK_XFRM)
	}
	// NETLINK_NETFILTER test
	if err := modprobe.LoadModules(context.TODO(), checkNfSocket, "nf_conntrack", "nf_conntrack_netlink"); err != nil {
		log.G(context.TODO()).Warnf("Could not load necessary modules for Conntrack: %v", err)
	} else {
		fams = append(fams, syscall.NETLINK_NETFILTER)
	}

	return fams
}

// API check on required xfrm modules (xfrm_user, xfrm_algo)
func checkXfrmSocket() error {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_XFRM)
	if err != nil {
		return err
	}
	syscall.Close(fd)
	return nil
}

// API check on required nf_conntrack* modules (nf_conntrack, nf_conntrack_netlink)
func checkNfSocket() error {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_NETFILTER)
	if err != nil {
		return err
	}
	syscall.Close(fd)
	return nil
}
