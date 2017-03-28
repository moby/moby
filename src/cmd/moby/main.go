package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
)

func main() {
	flag.Usage = func() {
		fmt.Printf("USAGE: %s [options] COMMAND\n\n", os.Args[0])
		fmt.Printf("Commands:\n")
		fmt.Printf("  build       Build a Moby image from a YAML file\n")
		fmt.Printf("  run         Run a Moby image on a local hypervisor\n")
		fmt.Printf("  help        Print this message\n")
		fmt.Printf("\n")
		fmt.Printf("Run '%s COMMAND --help' for more information on the command\n", os.Args[0])
		fmt.Printf("\n")
		fmt.Printf("Options:\n")
		flag.PrintDefaults()
	}
	flagQuiet := flag.Bool("q", false, "Quiet execution")
	flagVerbose := flag.Bool("v", false, "Verbose execution")

	buildCmd := flag.NewFlagSet("build", flag.ExitOnError)
	buildCmd.Usage = func() {
		fmt.Printf("USAGE: %s build [options] [file.yml]\n\n", os.Args[0])
		fmt.Printf("'file.yml' defaults to 'moby.yml' if not specified.\n\n")
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

	// Set up logging
	log.SetFormatter(&log.TextFormatter{})
	log.SetLevel(log.InfoLevel)
	flag.Parse()
	if *flagQuiet && *flagVerbose {
		fmt.Printf("Can't set quiet and verbose flag at the same time\n")
		os.Exit(1)
	}
	if *flagQuiet {
		log.SetLevel(log.ErrorLevel)
	}
	if *flagVerbose {
		log.SetLevel(log.DebugLevel)
	}

	args := flag.Args()
	if len(args) < 1 {
		fmt.Printf("Please specify a command.\n\n")
		flag.Usage()
		os.Exit(1)
	}

	switch args[0] {
	case "build":
		buildCmd.Parse(args[1:])
		build(*buildName, *buildPull, buildCmd.Args())
	case "run":
		runCmd.Parse(args[1:])
		run(*runCPUs, *runMem, *runDiskSz, *runDisk, runCmd.Args())
	case "help":
		flag.Usage()
	default:
		fmt.Printf("%q is not valid command.\n\n", args[0])
		flag.Usage()
		os.Exit(1)
	}
}
