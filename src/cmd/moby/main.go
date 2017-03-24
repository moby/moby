package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Usage = func() {
		fmt.Printf("USAGE: %s COMMAND\n\n", os.Args[0])
		fmt.Printf("Commands:\n")
		fmt.Printf("  build       Build a Moby image from a YAML file\n")
		fmt.Printf("  run         Run a Moby image on a local hypervisor\n")
		fmt.Printf("  help        Print this message\n")
		fmt.Printf("\n")
		fmt.Printf("Run '%s COMMAND --help' for more information on the command\n", os.Args[0])
	}

	buildCmd := flag.NewFlagSet("build", flag.ExitOnError)
	buildCmd.Usage = func() {
		fmt.Printf("USAGE: %s build [options] [file.yaml]\n\n", os.Args[0])
		fmt.Printf("'file.yaml' defaults to 'moby.yaml' if not specified.\n\n")
		fmt.Printf("Options:\n")
		buildCmd.PrintDefaults()
	}
	buildName := buildCmd.String("name", "", "Name to use for output files")
	buildPull := buildCmd.Bool("pull", false, "Always pull images")

	runCmd := flag.NewFlagSet("run", flag.ExitOnError)
	runCmd.Usage = func() {
		fmt.Printf("USAGE: %s run [options] [prefix]\n\n", os.Args[0])
		fmt.Printf("'prefix' specifies the path to the VM image.\n")
		fmt.Printf("It defaults to './moby'.\n")
		fmt.Printf("\n")
		fmt.Printf("Options:\n")
		runCmd.PrintDefaults()
		fmt.Printf("\n")
		fmt.Printf("If 'data' is supplied or if 'background' is selected\n")
		fmt.Printf("some per VM state is kept in a sub-directory in the ~/.moby\n")
	}
	runCPUs := runCmd.Int("cpus", 1, "Number of CPUs")
	runMem := runCmd.Int("mem", 1024, "Amount of memory in MB")
	runDiskSz := runCmd.Int("disk-size", 0, "Size of Disk in MB")
	runDisk := runCmd.String("disk", "", "Path to disk image to used")

	if len(os.Args) < 2 {
		fmt.Printf("Please specify a command.\n\n")
		flag.Usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		buildCmd.Parse(os.Args[2:])
		build(*buildName, *buildPull, buildCmd.Args())
	case "run":
		runCmd.Parse(os.Args[2:])
		run(*runCPUs, *runMem, *runDiskSz, *runDisk, runCmd.Args())
	case "help":
		flag.Usage()
	default:
		fmt.Printf("%q is not valid command.\n\n", os.Args[1])
		flag.Usage()
		os.Exit(1)
	}
}
