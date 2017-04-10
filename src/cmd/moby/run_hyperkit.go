package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/hyperkit/go"
	"github.com/rneugeba/iso9660wrap"
)

// Process the run arguments and execute run
func runHyperKit(args []string) {
	hyperkitCmd := flag.NewFlagSet("hyperkit", flag.ExitOnError)
	hyperkitCmd.Usage = func() {
		fmt.Printf("USAGE: %s run hyperkit [options] prefix\n\n", os.Args[0])
		fmt.Printf("'prefix' specifies the path to the VM image.\n")
		fmt.Printf("\n")
		fmt.Printf("Options:\n")
		hyperkitCmd.PrintDefaults()
	}
	hyperkitPath := hyperkitCmd.String("hyperkit", "", "Path to hyperkit binary (if not in default location)")
	cpus := hyperkitCmd.Int("cpus", 1, "Number of CPUs")
	mem := hyperkitCmd.Int("mem", 1024, "Amount of memory in MB")
	diskSz := hyperkitCmd.Int("disk-size", 0, "Size of Disk in MB")
	disk := hyperkitCmd.String("disk", "", "Path to disk image to used")
	data := hyperkitCmd.String("data", "", "Metadata to pass to VM (either a path to a file or a string)")
	ipStr := hyperkitCmd.String("ip", "", "IP address for the VM")

	hyperkitCmd.Parse(args)
	remArgs := hyperkitCmd.Args()
	if len(remArgs) == 0 {
		fmt.Println("Please specify the prefix to the image to boot\n")
		hyperkitCmd.Usage()
		os.Exit(1)
	}
	prefix := remArgs[0]

	isoPath := ""
	if *data != "" {
		var d []byte
		if _, err := os.Stat(*data); os.IsNotExist(err) {
			d = []byte(*data)
		} else {
			d, err = ioutil.ReadFile(*data)
			if err != nil {
				log.Fatalf("Cannot read user data: %v", err)
			}
		}
		isoPath = prefix + "-data.iso"
		outfh, err := os.OpenFile(isoPath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Cannot create user data ISO: %v", err)
		}
		err = iso9660wrap.WriteBuffer(outfh, d, "config")
		if err != nil {
			log.Fatalf("Cannot write user data ISO: %v", err)
		}
		outfh.Close()
	}

	uuidStr := ""
	if *ipStr != "" {
		// If an IP address was requested construct a "special" UUID
		// for the VM.
		if ip := net.ParseIP(*ipStr); len(ip) > 0 {
			uuid := make([]byte, 16)
			uuid[12] = ip.To4()[0]
			uuid[13] = ip.To4()[1]
			uuid[14] = ip.To4()[2]
			uuid[15] = ip.To4()[3]
			uuidStr = fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
		}
	}

	// Run
	cmdline, err := ioutil.ReadFile(prefix + "-cmdline")
	if err != nil {
		log.Fatalf("Cannot open cmdline file: %v", err)
	}

	if *diskSz != 0 && *disk == "" {
		*disk = prefix + "-disk.img"
	}

	h, err := hyperkit.New(*hyperkitPath, "auto", "")
	if err != nil {
		log.Fatalln("Error creating hyperkit: ", err)
	}

	h.Kernel = prefix + "-bzImage"
	h.Initrd = prefix + "-initrd.img"
	h.UUID = uuidStr
	h.DiskImage = *disk
	h.ISOImage = isoPath
	h.CPUs = *cpus
	h.Memory = *mem
	h.DiskSize = *diskSz

	err = h.Run(string(cmdline))
	if err != nil {
		log.Fatalf("Cannot run hyperkit: %v", err)
	}
}
