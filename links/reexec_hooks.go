package links

import (
	"flag"
	"log"
	"os"
	"syscall"

	"github.com/docker/docker/pkg/execin"
	"github.com/docker/docker/reexec"
)

func init() {
	reexec.Register("docker-network-movens", networkMoveNS)
}

func networkMoveNS() {
	var (
		frompid = flag.String("frompid", "", "")
		topid   = flag.String("topid", "", "")
		ipaddr  = flag.String("ip", "", "")
		gateway = flag.String("gateway", "", "")
		device  = flag.String("device", "eth0", "")
	)

	flag.Parse()

	f1, err := netNamespace(*frompid)
	if err != nil {
		log.Fatal(err)
	}

	f2, err := netNamespace(*topid)
	if err != nil {
		log.Fatal(err)
	}

	err = execin.ExecIn(
		f1,
		syscall.CLONE_NEWNET,
		moveVethDevice(
			f1, f2,
			NetworkSettings{
				IpNet:      *ipaddr,
				Gateway:    *gateway,
				DeviceName: *device,
			}))

	if err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}
