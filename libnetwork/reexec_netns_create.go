package libnetwork

import (
	"os"
	"runtime"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

func reexecCreateNamespace() {
	runtime.LockOSThread()

	if len(os.Args) < 2 {
		log.Fatalf("no namespace path provided")
	}

	if err := createNamespaceFile(os.Args[1]); err != nil {
		log.Fatal(err)
	}

	if err := syscall.Unshare(syscall.CLONE_NEWNET); err != nil {
		log.Fatal(err)
	}

	if err := loopbackUp(); err != nil {
		log.Fatal(err)
	}

	if err := syscall.Mount("/proc/self/ns/net", os.Args[1], "bind", syscall.MS_BIND, ""); err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}

func createNamespaceFile(path string) (err error) {
	var f *os.File
	if f, err = os.Create(path); err == nil {
		f.Close()
	}
	return err
}

func loopbackUp() error {
	iface, err := netlink.LinkByName("lo")
	if err != nil {
		return err
	}
	return netlink.LinkSetUp(iface)
}
