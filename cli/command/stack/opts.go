package stack

import (
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/cli/command/bundlefile"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

func addComposefileFlag(opt *string, flags *pflag.FlagSet) {
	flags.StringVarP(opt, "compose-file", "c", "", "Path to a Compose file")
	flags.SetAnnotation("compose-file", "version", []string{"1.25"})
}

func addBundlefileFlag(opt *string, flags *pflag.FlagSet) {
	flags.StringVar(opt, "bundle-file", "", "Path to a Distributed Application Bundle file")
	flags.SetAnnotation("bundle-file", "experimental", nil)
}

func addRegistryAuthFlag(opt *bool, flags *pflag.FlagSet) {
	flags.BoolVar(opt, "with-registry-auth", false, "Send registry authentication details to Swarm agents")
}

func loadBundlefile(stderr io.Writer, namespace string, path string) (*bundlefile.Bundlefile, error) {
	defaultPath := fmt.Sprintf("%s.dab", namespace)

	if path == "" {
		path = defaultPath
	}
	if _, err := os.Stat(path); err != nil {
		return nil, errors.Errorf(
			"Bundle %s not found. Specify the path with --file",
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
		return nil, errors.Errorf("Error reading %s: %v\n", path, err)
	}
	return bundle, err
}
