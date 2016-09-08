package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/cpuguy83/go-md2man/md2man"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/command"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/term"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func generateCobraManPages(outPath string) error {
	section1Path := filepath.Join(outPath, "man1")
	if err := os.MkdirAll(section1Path, 0755); err != nil {
		return err
	}
	header := &doc.GenManHeader{
		Title:   "DOCKER",
		Section: "1",
		Source:  "Docker Community",
	}

	stdin, stdout, stderr := term.StdStreams()
	dockerCli := client.NewDockerCli(stdin, stdout, stderr)
	cmd := &cobra.Command{Use: "docker"}
	command.AddCommands(cmd, dockerCli)

	cmd.DisableAutoGenTag = true
	return doc.GenManTreeFromOpts(cmd, doc.GenManTreeOptions{
		Header:           header,
		Path:             section1Path,
		CommandSeparator: "-",
	})
}

func generateLegacyManPages(outPath, inPath string) error {
	mdPaths, err := filepath.Glob(filepath.Join(inPath, "*.md"))
	if err != nil {
		return err
	}
	for _, mdPath := range mdPaths {
		re := regexp.MustCompile("([^\\.]*)\\.([0-9]+)\\.md")
		matches := re.FindStringSubmatch(filepath.Base(mdPath))
		if len(matches) != 3 {
			// skip non-manual files e.g. "README.md"
			continue
		}
		name, sectionNum := matches[1], matches[2]
		manPath := filepath.Join(outPath,
			fmt.Sprintf("man%s", sectionNum),
			fmt.Sprintf("%s.%s", name, sectionNum))
		if err = convertMarkdownToManPage(manPath, mdPath); err != nil {
			return err
		}
	}
	return nil
}

func convertMarkdownToManPage(manPath, mdPath string) error {
	md, err := ioutil.ReadFile(mdPath)
	if err != nil {
		return err
	}
	man := md2man.Render(md)
	if err = os.MkdirAll(filepath.Dir(manPath), 0755); err != nil {
		return err
	}
	return ioutil.WriteFile(manPath, man, 0644)
}

func main() {
	var opts struct {
		outPath string
		inPath  string
	}
	cmd := &cobra.Command{
		Use:   "[OPTIONS]",
		Short: "Generate manual pages",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Generating cobra man pages under %s\n", opts.outPath)
			if err := generateCobraManPages(opts.outPath); err != nil {
				return fmt.Errorf("failed to generate cobra man pages: %v", err)
			}
			fmt.Printf("Generating legacy man pages (for %s) under %s\n",
				opts.inPath, opts.outPath)
			if err := generateLegacyManPages(opts.outPath, opts.inPath); err != nil {
				return fmt.Errorf("failed to generate legacy man pages: %v", err)
			}
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.outPath, "out", "o", "/tmp", "Output directory path")
	flags.StringVarP(&opts.inPath, "in", "i", "./man", "Input directory path")
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(-1)
	}
}
