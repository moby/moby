package ns

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/internal/rootless"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/modprobe"
	"github.com/moby/moby/v2/daemon/libnetwork/nlwrap"
	"github.com/vishvananda/netns"
)

// NetlinkSocketsTimeout represents the default timeout duration for the sockets.
const NetlinkSocketsTimeout = 3 * time.Second

// initNamespace initializes a new network namespace.
var initNamespace = sync.OnceValues(initHandles)

// initHandles initializes a new network namespace
func initHandles() (netns.NsHandle, nlwrap.Handle) {
	var (
		initNs netns.NsHandle
		initNl nlwrap.Handle
		err    error
	)
	detachedNetNS, err := rootless.DetachedNetNS()
	if err != nil {
		log.G(context.Background()).WithError(err).Error("could not check for detached netns")
	}
	if detachedNetNS != "" {
		initNs, err = netns.GetFromPath(detachedNetNS)
		if err != nil {
			log.G(context.Background()).WithError(err).Errorf("could not get detached network namespace %s", detachedNetNS)
			return initNs, initNl
		}
		initNl, err = nlwrap.NewHandleAt(initNs, getSupportedNlFamilies()...)
	} else {
		initNs, err = netns.Get()
		if err != nil {
			log.G(context.Background()).WithError(err).Error("could not get initial namespace: falling back to using netns.None")
			initNs = netns.None()
		}
		initNl, err = nlwrap.NewHandle(getSupportedNlFamilies()...)
	}
	if err != nil {
		// Fail fast to keep the invariant: NlHandle must be a valid handle
		panic(fmt.Errorf("could not create netlink handle on initial (host) namespace: %w", err))
	}
	err = initNl.SetSocketTimeout(NetlinkSocketsTimeout)
	if err != nil {
		log.G(context.Background()).WithError(err).Warn("failed to set the timeout on the default netlink handle sockets")
	}

	return initNs, initNl
}

// ResetHandles resets the initial namespace and netlink handles.
// This is useful for testing to ensure a clean state. It will
// panic if called outside a test.
//
// Note: This function is not safe for concurrent use with callers
// that are using handles obtained from this package. It may close
// handles while they are still in use.
func ResetHandles() {
	if !testing.Testing() {
		panic("ResetHandles should only be called from tests")
	}
	initNs, initNl := initNamespace()
	// Reset the initNamespace sync.OnceValues. This may race with
	// concurrent callers still calling the old initNamespace (and
	// values), but adding a [sync.RWMutex] only for the test-case
	// is probably too much (unless things are racy).
	initNamespace = sync.OnceValues(initHandles)
	if initNs.IsOpen() {
		_ = initNs.Close()
	}
	if initNl.Handle != nil {
		initNl.Close()
	}
}

// NsHandle returns the network namespace handle for the initial (host) namespace.
func NsHandle() netns.NsHandle {
	ns, _ := initNamespace()
	return ns
}

// NlHandle returns the netlink handle.
func NlHandle() nlwrap.Handle {
	_, nl := initNamespace()
	return nl
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
	_ = syscall.Close(fd)
	return nil
}

// API check on required nf_conntrack* modules (nf_conntrack, nf_conntrack_netlink)
func checkNfSocket() error {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_NETFILTER)
	if err != nil {
		return err
	}
	_ = syscall.Close(fd)
	return nil
}
