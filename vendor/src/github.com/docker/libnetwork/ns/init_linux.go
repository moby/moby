package ns

import (
	"fmt"
	"os"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/vishvananda/netns"
)

var initNs netns.NsHandle

// Init initializes a new network namespace
func Init() {
	var err error
	initNs, err = netns.Get()
	if err != nil {
		log.Errorf("could not get initial namespace: %v", err)
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

// ParseHandlerInt transforms the namespace handler into a integer
func ParseHandlerInt() int {
	return int(initNs)
}

func getLink() (string, error) {
	return os.Readlink(fmt.Sprintf("/proc/%d/task/%d/ns/net", os.Getpid(), syscall.Gettid()))
}
