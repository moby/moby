package ns

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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
	initNl, err = netlink.NewHandle(getSupportedNlFamilies()...)
	if err != nil {
		log.Errorf("could not create netlink handle on initial namespace: %v", err)
	}
}

// SetNamespace sets the initial namespace handler
func SetNamespace() error {
	initOnce.Do(Init)
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

func getSupportedNlFamilies() []int {
	fams := []int{syscall.NETLINK_ROUTE}
	if err := loadXfrmModules(); err != nil {
		log.Warnf("Could not load necessary modules for IPSEC rules: %v", err)
		return fams
	}
	return append(fams, syscall.NETLINK_XFRM)
}

func loadXfrmModules() error {
	if out, err := exec.Command("modprobe", "-va", "xfrm_user").CombinedOutput(); err != nil {
		return fmt.Errorf("Running modprobe xfrm_user failed with message: `%s`, error: %v", strings.TrimSpace(string(out)), err)
	}
	if out, err := exec.Command("modprobe", "-va", "xfrm_algo").CombinedOutput(); err != nil {
		return fmt.Errorf("Running modprobe xfrm_algo failed with message: `%s`, error: %v", strings.TrimSpace(string(out)), err)
	}
	return nil
}
