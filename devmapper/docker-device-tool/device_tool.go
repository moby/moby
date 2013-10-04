package main

import (
	"fmt"
	"github.com/dotcloud/docker/devmapper"
	"os"
)

func usage() {
	fmt.Printf("Usage: %s [snap new-id base-id] | [remove id] | [mount id mountpoint]\n", os.Args[0])
	os.Exit(1)
}

func main() {
	devices := devmapper.NewDeviceSetDM("/var/lib/docker")

	if len(os.Args) < 2 {
		usage()
	}

	cmd := os.Args[1]
	if cmd == "snap" {
		if len(os.Args) < 4 {
			usage()
		}

		err := devices.AddDevice(os.Args[2], os.Args[3])
		if err != nil {
			fmt.Println("Can't create snap device: ", err)
			os.Exit(1)
		}
	} else if cmd == "remove" {
		if len(os.Args) < 3 {
			usage()
		}

		err := devices.RemoveDevice(os.Args[2])
		if err != nil {
			fmt.Println("Can't remove device: ", err)
			os.Exit(1)
		}
	} else if cmd == "mount" {
		if len(os.Args) < 4 {
			usage()
		}

		err := devices.MountDevice(os.Args[2], os.Args[3])
		if err != nil {
			fmt.Println("Can't create snap device: ", err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("Unknown command %s\n", cmd)
		if len(os.Args) < 4 {
			usage()
		}

		os.Exit(1)
	}

	return
}
