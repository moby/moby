package main

import (
	"fmt"
	"os"
	"runtime"

	flag "github.com/docker/docker/pkg/mflag"
)

func init() {
}

func getHomeDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}

var (
	flPurgeCache = flag.Bool([]string{"-purge-cache"}, false, "Purges the pulldocker cache after this command is run (saves space when you plan to pull only one image)")
	flReadOnly = flag.Bool([]string{"-read-only"}, false, "Mount binds the image read-only from the pulldocker cache to the output directory")
	flLogLevel    = flag.String([]string{"l", "-log-level"}, "info", "Set the logging level")
	flDebug       = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
	flVersion     = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")

	flOutputDir = flag.String([]string{"o", "-output-dir"}, ".", "Directory to dump image")
)

func init() {

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: pulldocker [-o DIR] NAME[:TAG]\n\nPull an image from the registry and dump the files flat. Saves the layer cache in ~/pulldockercache.\n\n")

		flag.PrintDefaults()
	}
}
