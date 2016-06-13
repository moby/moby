package ns

import (
	"fmt"
	"os"
	"sync"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

var (
	initNs   netns.NsHandle
	initNl   *netlink.Handle
	initOnce sync.Once
)

// Init initializes a new network namespace
func Init() {
	var err error
	initNs, err = netns.Get()
	if err != nil {
		log.Errorf("could not get initial namespace: %v", err)
	}
	initNl, err = netlink.NewHandle()
	if err != nil {
		log.Errorf("could not create netlink handle on initial namespace: %v", err)
	}
}

// SetNamespace sets the initial namespace handler
func SetNamespace() error {
	if err := netns.Set(initNs); err != nil {
		linkInfo, linkErr := getLink()
		if linkErr != nil {
			linkInfo = linkErr.Error()
		}
		return fmt.Errorf("failed to set to initial namespace, %v, initns fd %d: %v", linkInfo, initNs, err)
	}
	return nil
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

func getLink() (string, error) {
	return os.Readlink(fmt.Sprintf("/proc/%d/task/%d/ns/net", os.Getpid(), syscall.Gettid()))
}

// NlHandle returns the netlink handler
func NlHandle() *netlink.Handle {
	initOnce.Do(Init)
	return initNl
}
