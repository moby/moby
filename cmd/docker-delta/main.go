package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/moby/moby/client"
	"github.com/spf13/cobra"
)

func main() {
	var tag string

	cmd := &cobra.Command{
		Use:   "docker-delta BASE_IMAGE TARGET_IMAGE",
		Short: "Create a binary delta between two images",
		Long: `Create a binary delta between two images using librsync.

The delta is stored as an OCI image with special labels:
  - io.resin.delta.base: The base image ID
  - io.resin.delta.config: JSON metadata about the delta

Example:
  docker-delta busybox:1.35.0 busybox:1.37.0 --tag my-delta:latest
`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseImage := args[0]
			targetImage := args[1]

			if tag == "" {
				return fmt.Errorf("--tag is required")
			}

			ctx := context.Background()
			cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err != nil {
				return fmt.Errorf("failed to create Docker client: %w", err)
			}
			defer cli.Close()

			fmt.Printf("Creating delta from %s to %s...\n", baseImage, targetImage)
			fmt.Printf("Output tag: %s\n", tag)

			rc, err := cli.ImageDelta(ctx, baseImage, targetImage, client.ImageDeltaOptions{
				Tag: tag,
			})
			if err != nil {
				return fmt.Errorf("failed to create delta: %w", err)
			}
			defer rc.Close()

			// Stream the progress output
			_, err = io.Copy(os.Stdout, rc)
			if err != nil {
				return fmt.Errorf("error reading response: %w", err)
			}

			fmt.Printf("\nDelta image created successfully: %s\n", tag)
			return nil
		},
	}

	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Tag for the delta image (required)")
	cmd.MarkFlagRequired("tag")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
