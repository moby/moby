package main

import (
	"fmt"
	"os"

	"github.com/docker/docker/cli/cobraadaptor"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/spf13/cobra/doc"
)

func generateManPages(path string) error {
	header := &doc.GenManHeader{
		Title:   "DOCKER",
		Section: "1",
		Source:  "Docker Community",
	}
	flags := &cliflags.ClientFlags{
		Common: cliflags.InitCommonFlags(),
	}
	cmd := cobraadaptor.NewCobraAdaptor(flags).GetRootCommand()
	cmd.DisableAutoGenTag = true
	return doc.GenManTreeFromOpts(cmd, doc.GenManTreeOptions{
		Header:           header,
		Path:             path,
		CommandSeparator: "-",
	})
}

func main() {
	path := "/tmp"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	fmt.Printf("Generating man pages into %s\n", path)
	if err := generateManPages(path); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate man pages: %s\n", err.Error())
	}
}
