package manifest

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type annotateOptions struct {
	target      string // the target manifest list name (also transaction ID)
	image       string // the manifest to annotate within the list
	variant     string // an architecture variant
	os          string
	arch        string
	cpuFeatures []string
	osFeatures  []string
}

// NewAnnotateCommand creates a new `docker manifest annotate` command
func newAnnotateCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts annotateOptions

	cmd := &cobra.Command{
		Use:   "annotate NAME[:TAG] [OPTIONS]",
		Short: "Add additional information to an image's manifest.",
		Args:  cli.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.target = args[0]
			opts.image = args[1]
			return runManifestAnnotate(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&opts.os, "os", "", "Add ios info to a manifest before pushing it.")
	flags.StringVar(&opts.arch, "arch", "", "Add arch info to a manifest before pushing it.")
	flags.StringSliceVar(&opts.cpuFeatures, "cpuFeatures", []string{}, "Add feature info to a manifest before pushing it.")
	flags.StringSliceVar(&opts.osFeatures, "osFeatures", []string{}, "Add feature info to a manifest before pushing it.")
	flags.StringVar(&opts.variant, "variant", "", "Add arch variant to a manifest before pushing it.")

	return cmd
}

func runManifestAnnotate(dockerCli *command.DockerCli, opts annotateOptions) error {

	// Make sure the manifests are pulled, find the file you need, unmarshal the json, edit the file, and done.
	targetRef, err := reference.ParseNormalizedNamed(opts.target)
	if err != nil {
		return fmt.Errorf("Annotate: Error parsing name for manifest list (%s): %s", opts.target, err)
	}
	imgRef, err := reference.ParseNormalizedNamed(opts.image)
	if err != nil {
		return fmt.Errorf("Annotate: Error prasing name for manifest (%s): %s:", opts.image, err)
	}

	// Make sure we've got tags or digests:
	if _, isDigested := targetRef.(reference.Canonical); !isDigested {
		targetRef = reference.TagNameOnly(targetRef)
	}
	if _, isDigested := imgRef.(reference.Canonical); !isDigested {
		imgRef = reference.TagNameOnly(imgRef)
	}
	transactionID := makeFilesafeName(targetRef.String())
	imgID := makeFilesafeName(imgRef.String())
	logrus.Debugf("Beginning annotate for %s/%s", transactionID, imgID)

	imgInspect, _, err := getImageData(dockerCli, imgRef.String(), targetRef.String(), false)
	if err != nil {
		return err
	}

	if len(imgInspect) > 1 {
		return fmt.Errorf("Cannot annotate manifest list. Please pass an image (not list) name")
	}

	mf := imgInspect[0]

	newMf, err := unmarshalIntoManifestInspect(imgID, transactionID)
	if err != nil {
		return err
	}

	// Update the mf
	if opts.os != "" {
		newMf.OS = opts.os
	}
	if opts.arch != "" {
		newMf.Architecture = opts.arch
	}
	for _, cpuFeature := range opts.cpuFeatures {
		newMf.Features = appendIfUnique(mf.Features, cpuFeature)
	}
	for _, osFeature := range opts.osFeatures {
		newMf.OSFeatures = appendIfUnique(mf.OSFeatures, osFeature)
	}
	if opts.variant != "" {
		newMf.Variant = opts.variant
	}

	// validate os/arch input
	if !isValidOSArch(newMf.OS, newMf.Architecture) {
		return fmt.Errorf("Manifest entry for image has unsupported os/arch combination: %s/%s", opts.os, opts.arch)
	}
	// @TODO
	// dgst := digest.FromBytes(b) can't use b/c not of the json.

	if err := updateMfFile(newMf, imgID, transactionID); err != nil {
		return err
	}

	logrus.Debugf("Annotated %s with options %v", mf.RefName, opts)
	return nil
}
func appendIfUnique(list []string, str string) []string {
	for _, s := range list {
		if s == str {
			return list
		}
	}
	return append(list, str)
}
