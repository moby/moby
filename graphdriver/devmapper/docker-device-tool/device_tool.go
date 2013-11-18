package main

import (
	"flag"
	"fmt"
	"github.com/dotcloud/docker/graphdriver/devmapper"
	"os"
	"path"
	"sort"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <flags>  [status] | [list] | [device id] | [snap new-id base-id] | [remove id] | [mount id mountpoint]\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(1)
}

func main() {
	root := flag.String("r", "/var/lib/docker", "Docker root dir")
	flDebug := flag.Bool("D", false, "Debug mode")

	flag.Parse()

	if *flDebug {
		os.Setenv("DEBUG", "1")
	}

	if flag.NArg() < 1 {
		usage()
	}

	args := flag.Args()

	home := path.Join(*root, "devicemapper")
	devices, err := devmapper.NewDeviceSet(home, false)
	if err != nil {
		fmt.Println("Can't initialize device mapper: ", err)
		os.Exit(1)
	}

	switch args[0] {
	case "status":
		status := devices.Status()
		fmt.Printf("Pool name: %s\n", status.PoolName)
		fmt.Printf("Data Loopback file: %s\n", status.DataLoopback)
		fmt.Printf("Metadata Loopback file: %s\n", status.MetadataLoopback)
		fmt.Printf("Sector size: %d\n", status.SectorSize)
		fmt.Printf("Data use: %d of %d (%.1f %%)\n", status.Data.Used, status.Data.Total, 100.0*float64(status.Data.Used)/float64(status.Data.Total))
		fmt.Printf("Metadata use: %d of %d (%.1f %%)\n", status.Metadata.Used, status.Metadata.Total, 100.0*float64(status.Metadata.Used)/float64(status.Metadata.Total))
		break
	case "list":
		ids := devices.List()
		sort.Strings(ids)
		for _, id := range ids {
			fmt.Println(id)
		}
		break
	case "device":
		if flag.NArg() < 2 {
			usage()
		}
		status, err := devices.GetDeviceStatus(args[1])
		if err != nil {
			fmt.Println("Can't get device info: ", err)
			os.Exit(1)
		}
		fmt.Printf("Id: %d\n", status.DeviceId)
		fmt.Printf("Size: %d\n", status.Size)
		fmt.Printf("Transaction Id: %d\n", status.TransactionId)
		fmt.Printf("Size in Sectors: %d\n", status.SizeInSectors)
		fmt.Printf("Mapped Sectors: %d\n", status.MappedSectors)
		fmt.Printf("Highest Mapped Sector: %d\n", status.HighestMappedSector)
		break
	case "snap":
		if flag.NArg() < 3 {
			usage()
		}

		err := devices.AddDevice(args[1], args[2])
		if err != nil {
			fmt.Println("Can't create snap device: ", err)
			os.Exit(1)
		}
		break
	case "remove":
		if flag.NArg() < 2 {
			usage()
		}

		err := devices.RemoveDevice(args[1])
		if err != nil {
			fmt.Println("Can't remove device: ", err)
			os.Exit(1)
		}
		break
	case "mount":
		if flag.NArg() < 3 {
			usage()
		}

		err := devices.MountDevice(args[1], args[2], false)
		if err != nil {
			fmt.Println("Can't create snap device: ", err)
			os.Exit(1)
		}
		break
	default:
		fmt.Printf("Unknown command %s\n", args[0])
		usage()

		os.Exit(1)
	}

	return
}
