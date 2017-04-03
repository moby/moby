package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/registry"
)

type inspectOptions struct {
	remote  string
	verbose bool
}

// NewInspectCommand creates a new `docker manifest inspect` command
func newInspectCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts inspectOptions

	cmd := &cobra.Command{
		Use:   "inspect [OPTIONS] NAME[:TAG]",
		Short: "Display an image's manifest, or a remote manifest list.",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.remote = args[0]
			return runListInspect(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(&opts.verbose, "verbose", "v", false, "Output additional info including layers and platform")

	return cmd
}

func runListInspect(dockerCli *command.DockerCli, opts inspectOptions) error {

	// Get the data and then format it
	var (
		imgInspect []ImgManifestInspect
		prettyJSON bytes.Buffer
	)

	named, err := reference.ParseNormalizedNamed(opts.remote)
	if err != nil {
		return err
	}
	targetRepo, err := registry.ParseRepositoryInfo(named)
	if err != nil {
		return err
	}

	// For now, always pull as there' no reason to store an inspect. They're quick to get.
	// When the engine is multi-arch image aware, we can store these in a universal location to
	// save a little bandwidth.
	imgInspect, _, err = getImageData(dockerCli, named.String(), "", true)
	if err != nil {
		logrus.Fatal(err)
	}
	// output basic informative details about the image
	if len(imgInspect) == 1 {
		// this is a basic single manifest
		err = json.Indent(&prettyJSON, imgInspect[0].CanonicalJSON, "", "    ")
		if err != nil {
			return err
		}
		fmt.Fprintf(dockerCli.Out(), "%s\n", prettyJSON.String())
		if !opts.verbose {
			return nil
		}
		mfd, _, err := buildManifestObj(targetRepo, imgInspect[0])
		if err != nil {
			return err
		}
		jsonBytes, err := json.Marshal(mfd)
		if err != nil {
			return err
		}
		prettyJSON.Reset()
		err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
		if err != nil {
			return err
		}
		fmt.Fprintf(dockerCli.Out(), "%s\n", prettyJSON.String())
		return nil
	}

	manifests := []manifestlist.ManifestDescriptor{}
	// More than one response. This is a manifest list.
	for _, img := range imgInspect {
		mfd, _, err := buildManifestObj(targetRepo, img)
		if err != nil {
			return fmt.Errorf("Error assembling ManifestDescriptor")
		}
		manifests = append(manifests, mfd)
	}
	deserializedML, err := manifestlist.FromDescriptors(manifests)
	if err != nil {
		return err
	}
	jsonBytes, err := deserializedML.MarshalJSON()
	if err != nil {
		return err
	}
	err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
	if err != nil {
		return err
	}
	fmt.Fprintf(dockerCli.Out(), "%s\n", prettyJSON.String())
	if !opts.verbose {
		return nil
	}
	for _, img := range imgInspect {
		switch img.MediaType {
		case schema1.MediaTypeManifest:
			var manifestv1 schema1.Manifest
			err := json.Unmarshal(img.CanonicalJSON, &manifestv1)
			if err != nil {
				return err
			}
			jsonBytes, err := json.Marshal(manifestv1)
			if err != nil {
				return err
			}
			prettyJSON.Reset()
			err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
			if err != nil {
				return err
			}
			fmt.Fprintf(dockerCli.Out(), "%s\n", prettyJSON.String())
		case schema2.MediaTypeManifest:
			var manifestv2 schema2.Manifest
			err := json.Unmarshal(img.CanonicalJSON, &manifestv2)
			if err != nil {
				return err
			}
			jsonBytes, err := json.Marshal(manifestv2)
			if err != nil {
				return err
			}
			prettyJSON.Reset()
			err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
			if err != nil {
				return err
			}
			fmt.Fprintf(dockerCli.Out(), "%s\n", prettyJSON.String())
		}
		/*
			prettyJSON.Reset()
			err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
			if err != nil {
				return err
			}
			fmt.Fprintf(dockerCli.Out(), "%s\n", prettyJSON.String())
		*/
	}
	return nil
}
