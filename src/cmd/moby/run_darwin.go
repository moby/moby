// +build darwin

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
func run(args []string) {
	runCmd := flag.NewFlagSet("run", flag.ExitOnError)
	runCmd.Usage = func() {
		fmt.Printf("USAGE: %s run [options] [prefix]\n\n", os.Args[0])
		fmt.Printf("'prefix' specifies the path to the VM image.\n")
		fmt.Printf("It defaults to './moby'.\n")
		fmt.Printf("\n")
		fmt.Printf("Options:\n")
		runCmd.PrintDefaults()
	}
	runCPUs := runCmd.Int("cpus", 1, "Number of CPUs")
	runMem := runCmd.Int("mem", 1024, "Amount of memory in MB")
	runDiskSz := runCmd.Int("disk-size", 0, "Size of Disk in MB")
	runDisk := runCmd.String("disk", "", "Path to disk image to used")

	runCmd.Parse(args)
	remArgs := runCmd.Args()

	prefix := "moby"
	if len(remArgs) > 0 {
		prefix = remArgs[0]
	}

	runInternal(*runCPUs, *runMem, *runDiskSz, *runDisk, prefix)
}

func runInternal(cpus, mem, diskSz int, disk, prefix string) {
	cmdline, err := ioutil.ReadFile(prefix + "-cmdline")
	if err != nil {
		log.Fatalf("Cannot open cmdline file: %v", err)
	}

	if diskSz != 0 && disk == "" {
		disk = prefix + "-disk.img"
	}

	h, err := hyperkit.New("", "", "auto", disk)
	if err != nil {
		log.Fatalln("Error creating hyperkit: ", err)
	}

	h.Kernel = prefix + "-bzImage"
	h.Initrd = prefix + "-initrd.img"
	h.CPUs = cpus
	h.Memory = mem
	h.DiskSize = diskSz

	err = h.Run(string(cmdline))
	if err != nil {
		log.Fatalf("Cannot run hyperkit: %v", err)
	}
}
