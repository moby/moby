package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"

	log "github.com/Sirupsen/logrus"
)

//Version 12 relates to Fusion 8 and WS 12
//virtualHW.version = "12"

const vmxHW string = `config.version = "8"
virtualHW.version = "12"
vmci0.present = "TRUE"
floppy0.present = "FALSE"
displayName = "%s"
numvcpus = "%d"
memsize = "%d"
`

const vmxDisk string = `scsi0.present = "TRUE"
scsi0.sharedBus = "none"
scsi0.virtualDev = "lsilogic"
scsi0:0.present = "TRUE"
scsi0:0.fileName = "%s"
scsi0:0.deviceType = "scsi-hardDisk"
`

const vmxCdrom string = `ide1:0.present = "TRUE"
ide1:0.fileName = "%s"
ide1:0.deviceType = "cdrom-image"
`

const vmxPCI string = `pciBridge0.present = "TRUE"
pciBridge4.present = "TRUE"
pciBridge4.virtualDev = "pcieRootPort"
pciBridge4.functions = "8"
pciBridge5.present = "TRUE"
pciBridge5.virtualDev = "pcieRootPort"
pciBridge5.functions = "8"
pciBridge6.present = "TRUE"
pciBridge6.virtualDev = "pcieRootPort"
pciBridge6.functions = "8"
pciBridge7.present = "TRUE"
pciBridge7.virtualDev = "pcieRootPort"
pciBridge7.functions = "8"
ethernet0.pciSlotNumber = "32"
ethernet0.present = "TRUE"
ethernet0.virtualDev = "e1000"
ethernet0.networkName = "Inside"
ethernet0.generatedAddressOffset = "0"
guestOS = "other3xlinux-64"
`

func runVMware(args []string) {
	vmwareArgs := flag.NewFlagSet("vmware", flag.ExitOnError)
	vmwareArgs.Usage = func() {
		fmt.Printf("USAGE: %s run vmware [options] prefix\n\n", os.Args[0])
		fmt.Printf("'prefix' specifies the path to the VM image.\n")
		fmt.Printf("\n")
		fmt.Printf("Options:\n")
		vmwareArgs.PrintDefaults()
	}
	runCPUs := vmwareArgs.Int("cpus", 1, "Number of CPUs")
	runMem := vmwareArgs.Int("mem", 1024, "Amount of memory in MB")
	runDisk := vmwareArgs.String("disk", "", "Path to disk image to use")

	if err := vmwareArgs.Parse(args); err != nil {
		log.Fatal("Unable to parse args")
	}
	remArgs := vmwareArgs.Args()

	if len(remArgs) == 0 {
		fmt.Println("Please specify the prefix to the image to boot")
		vmwareArgs.Usage()
		os.Exit(1)
	}
	prefix := remArgs[0]

	// Build the contents of the VMWare .vmx file
	vmx := buildVMX(*runCPUs, *runMem, *runDisk, prefix)

	if vmx == "" {
		log.Fatalf("VMware .vmx file could not be generated, please confirm inputs")
	}

	var path string
	var vmrunArgs []string

	if runtime.GOOS == "windows" {
		path = "C:\\Program\\ files\\VMware Workstation\\vmrun.exe"
		vmrunArgs = []string{"-T", "ws", "start", prefix + ".vmx"}
	}

	if runtime.GOOS == "darwin" {
		path = "/Applications/VMware Fusion.app/Contents/Library/vmrun"
		vmrunArgs = []string{"-T", "fusion", "start", prefix + ".vmx"}
	}

	if runtime.GOOS == "linux" {
		path = "vmrun"
		fullVMrunPath, err := exec.LookPath(path)
		if err != nil {
			log.Fatalf("Unable to find %s within the $PATH", path)
		}
		path = fullVMrunPath
		vmrunArgs = []string{"-T", "ws", "start", prefix + ".vmx"}
	}

	// Check executables exist before attempting to execute
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Fatalf("ERROR VMware exectuables can not be found, ensure software is installed")
	}

	// Create the .vmx file
	err := ioutil.WriteFile(prefix+".vmx", []byte(vmx), 0644)

	if err != nil {
		log.Fatalf("Error writing .vmx file")
	}

	cmd := exec.Command(path, vmrunArgs...)
	out, err := cmd.Output()

	if err != nil {
		log.Fatalf("Error starting vmrun")
	}

	// check there is output to push to logging
	if len(out) > 0 {
		log.Info(out)
	}
}

func buildVMX(cpus int, mem int, diskPath string, prefix string) string {
	// CD-ROM can be added for use in a further release
	cdromPath := ""

	var returnString string

	returnString += fmt.Sprintf(vmxHW, prefix, cpus, mem)

	if cdromPath != "" {
		returnString += fmt.Sprintf(vmxCdrom, cdromPath)
	}

	if diskPath != "" {
		returnString += fmt.Sprintf(vmxDisk, diskPath)
	}

	returnString += vmxPCI
	return returnString
}
