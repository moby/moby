package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	log "github.com/Sirupsen/logrus"
)

func runQemu(args []string) {
	qemuFlags := flag.NewFlagSet("qemu", flag.ExitOnError)
	qemuFlags.Usage = func() {
		fmt.Printf("USAGE: %s run qemu [options] prefix\n\n", os.Args[0])
		fmt.Printf("'prefix' specifies the path to the VM image.\n")
		fmt.Printf("\n")
		fmt.Printf("Options:\n")
		qemuFlags.PrintDefaults()
	}

	// Determine Flags
	qemuGUI := qemuFlags.Bool("gui", false, "Set qemu to use video output instead of stdio")
	qemuUEFI := qemuFlags.Bool("uefi", false, "Set UEFI boot from 'prefix'-efi.iso")
	qemuIso := qemuFlags.Bool("iso", false, "Set Legacy BIOS boot from 'prefix'.iso")
	qemuKernel := qemuFlags.Bool("kernel", true, "Set boot using 'prefix'-bzImage/-initrd/-cmdline")

	// Paths and settings for Disks and UEFI firware
	qemuDiskPath := qemuFlags.String("diskpath", "", "Path to disk image to use")
	qemuDiskSize := qemuFlags.String("disksize", "", "Size of disk to create, only created if it doesn't exist")
	qemuFWPath := qemuFlags.String("fwpath", "/usr/share/ovmf/bios.bin", "Path to OVMF firmware for UEFI boot")

	// VM configuration
	qemuArch := qemuFlags.String("arch", "x86_64", "Type of architecture to use, e.g. x86_64, aarch64")
	qemuCPUs := qemuFlags.String("cpus", "1", "Number of CPUs")
	qemuMem := qemuFlags.String("mem", "1024", "Amount of memory in MB")

	qemuFlags.Parse(args)
	remArgs := qemuFlags.Args()

	if len(remArgs) == 0 {
		fmt.Println("Please specify the prefix to the image to boot")
		qemuFlags.Usage()
		os.Exit(1)
	}
	prefix := remArgs[0]

	// Print warning if conflicting UEFI and ISO flags are set
	if *qemuUEFI && *qemuIso {
		log.Warnf("Both -iso and -uefi have been used")
	}

	// Before building qemu arguments, check qemu is in the $PATH
	qemuBinPath := "qemu-system-" + *qemuArch
	qemuImgPath := "qemu-img"
	fullQemuPath, err := exec.LookPath(qemuBinPath)
	if err != nil {
		log.Fatalf("Unable to find %s within the $PATH", qemuBinPath)
	}

	// Iterate through the flags and build arguments
	var qemuArgs []string
	qemuArgs = append(qemuArgs, "-device", "virtio-rng-pci")
	qemuArgs = append(qemuArgs, "-smp", *qemuCPUs)
	qemuArgs = append(qemuArgs, "-m", *qemuMem)

	// Look for kvm device and enable for qemu if it exists
	if _, err = os.Stat("/dev/kvm"); os.IsNotExist(err) {
		qemuArgs = append(qemuArgs, "-machine", "q35")
	} else {
		qemuArgs = append(qemuArgs, "-enable-kvm")
		qemuArgs = append(qemuArgs, "-machine", "q35,accel=kvm:tcg")
	}

	if *qemuDiskPath != "" {
		// If disk doesn't exist then create one
		if _, err = os.Stat(*qemuDiskPath); os.IsNotExist(err) {
			log.Infof("Creating new qemu disk [%s]", *qemuDiskPath)
			fullQemuImgPath, err := exec.LookPath(qemuImgPath)
			if err != nil {
				log.Fatalf("Unable to find %s within the $PATH", qemuImgPath)
			}
			cmd := exec.Command(fullQemuImgPath, "create", "-f", "qcow2", *qemuDiskPath, *qemuDiskSize)
			cmd.Run()
		} else {
			log.Infof("Using existing disk [%s]", *qemuDiskPath)
		}
		qemuArgs = append(qemuArgs, "-disk", "file="+*qemuDiskPath+",format=raw")
	}

	// Check flags for iso/uefi boot and if so disable kernel boot
	if *qemuIso {
		*qemuKernel = false
		qemuIsoPath := buildPath(prefix, ".iso")
		qemuArgs = append(qemuArgs, "-cdrom", qemuIsoPath)
	}

	if *qemuUEFI {
		// Check for OVMF firmware before building paths
		_, err = os.Stat(*qemuFWPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Fatalf("File [%s] does not exist, please ensure OVMF is installed", *qemuFWPath)
			} else {
				log.Fatalf("%s", err.Error())
			}
		}

		*qemuKernel = false
		qemuIsoPath := buildPath(prefix, "-efi.iso")
		qemuArgs = append(qemuArgs, "-pflash", *qemuFWPath)
		qemuArgs = append(qemuArgs, "-cdrom", qemuIsoPath)
		qemuArgs = append(qemuArgs, "-boot", "d")
	}

	// build kernel boot config from bzImage/initrd/cmdline
	if *qemuKernel {
		qemuKernelPath := buildPath(prefix, "-bzImage")
		qemuInitrdPath := buildPath(prefix, "-initrd.img")
		qemuArgs = append(qemuArgs, "-kernel", qemuKernelPath)
		qemuArgs = append(qemuArgs, "-initrd", qemuInitrdPath)
		consoleString, err := ioutil.ReadFile(prefix + "-cmdline")
		if err != nil {
			log.Infof(" %s\n defaulting to console output", err.Error())
			qemuArgs = append(qemuArgs, "-append", "console=ttyS0 console=tty0 page_poison=1")
		} else {
			qemuArgs = append(qemuArgs, "-append", string(consoleString))
		}
	}

	if *qemuGUI != true {
		qemuArgs = append(qemuArgs, "-nographic")
	}

	// If verbosity is enabled print out the full path/arguments
	log.Debugf("%s %v\n", fullQemuPath, qemuArgs)

	cmd := exec.Command(fullQemuPath, qemuArgs...)

	// If we're not using a seperate window then link the execution to stdin/out
	if *qemuGUI != true {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	err = cmd.Run()

	if err != nil {
		log.Fatalf("Error starting %s\nError: %s", fullQemuPath, err.Error())
	}
}

func buildPath(prefix string, postfix string) string {
	path := prefix + postfix
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Fatalf("File [%s] does not exist in current directory", path)
	}
	return path
}
