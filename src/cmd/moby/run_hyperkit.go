package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/hyperkit/go"
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

	hyperkitCmd.Parse(args)
	remArgs := hyperkitCmd.Args()
	if len(remArgs) == 0 {
		fmt.Println("Please specify the prefix to the image to boot\n")
		hyperkitCmd.Usage()
		os.Exit(1)
	}
	prefix := remArgs[0]

	// Run
	cmdline, err := ioutil.ReadFile(prefix + "-cmdline")
	if err != nil {
		log.Fatalf("Cannot open cmdline file: %v", err)
	}

	if *diskSz != 0 && *disk == "" {
		*disk = prefix + "-disk.img"
	}

	h, err := hyperkit.New(*hyperkitPath, "", "auto", *disk)
	if err != nil {
		log.Fatalln("Error creating hyperkit: ", err)
	}

	h.Kernel = prefix + "-bzImage"
	h.Initrd = prefix + "-initrd.img"
	h.CPUs = *cpus
	h.Memory = *mem
	h.DiskSize = *diskSz

	err = h.Run(string(cmdline))
	if err != nil {
		log.Fatalf("Cannot run hyperkit: %v", err)
	}
}
