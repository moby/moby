package ns

import (
	"context"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/internal/modprobe"
	"github.com/docker/docker/internal/nlwrap"
	"github.com/vishvananda/netns"
)

var (
	initNs   netns.NsHandle
	initNl   nlwrap.Handle
	initOnce sync.Once
	// NetlinkSocketsTimeout represents the default timeout duration for the sockets
	NetlinkSocketsTimeout = 3 * time.Second
)

// Init initializes a new network namespace
func Init() {
	var err error
	initNs, err = netns.Get()
	if err != nil {
		log.G(context.TODO()).Errorf("could not get initial namespace: %v", err)
	}
	initNl, err = nlwrap.NewHandle(getSupportedNlFamilies()...)
	if err != nil {
		log.G(context.TODO()).Errorf("could not create netlink handle on initial namespace: %v", err)
	}
	err = initNl.SetSocketTimeout(NetlinkSocketsTimeout)
	if err != nil {
		log.G(context.TODO()).Warnf("Failed to set the timeout on the default netlink handle sockets: %v", err)
	}
}

// ParseHandlerInt transforms the namespace handler into an integer
func ParseHandlerInt() int {
	return int(getHandler())
}

// GetHandler returns the namespace handler
func getHandler() netns.NsHandle {
	initOnce.Do(Init)
	return initNs
}

// NlHandle returns the netlink handler
func NlHandle() nlwrap.Handle {
	initOnce.Do(Init)
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
