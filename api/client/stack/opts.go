// +build experimental

package stack

import (
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/client/bundlefile"
	"github.com/spf13/pflag"
)

func addBundlefileFlag(opt *string, flags *pflag.FlagSet) {
	flags.StringVarP(
		opt,
		"bundle", "f", "",
		"Path to a Distributed Application Bundle file (Default: STACK.dab)")
}

func loadBundlefile(stderr io.Writer, namespace string, path string) (*bundlefile.Bundlefile, error) {
	defaultPath := fmt.Sprintf("%s.dab", namespace)

	if path == "" {
		path = defaultPath
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf(
			"Bundle %s not found. Specify the path with -f or --bundle",
			path)
	}

	fmt.Fprintf(stderr, "Loading bundle from %s\n", path)
	reader, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	bundle, err := bundlefile.LoadFile(reader)
	if err != nil {
		return nil, fmt.Errorf("Error reading %s: %v\n", path, err)
	}
	return bundle, err
}
