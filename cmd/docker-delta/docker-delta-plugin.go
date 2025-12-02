package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/moby/moby/client"
	"github.com/spf13/cobra"
)

// Plugin metadata for Docker CLI
type PluginMetadata struct {
	SchemaVersion    string `json:"SchemaVersion"`
	Vendor           string `json:"Vendor"`
	Version          string `json:"Version"`
	ShortDescription string `json:"ShortDescription"`
	URL              string `json:"URL,omitempty"`
}

func main() {
	var tag string

	rootCmd := &cobra.Command{
		Use:   "docker-delta",
		Short: "Docker CLI plugin for creating binary deltas between images",
	}

	// Metadata command (required for Docker CLI plugins)
	metadataCmd := &cobra.Command{
		Use:    "docker-cli-plugin-metadata",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			metadata := PluginMetadata{
				SchemaVersion:    "0.1.0",
				Vendor:           "Balena",
				Version:          "0.1.0",
				ShortDescription: "Create binary deltas between Docker images",
				URL:              "https://github.com/balena-os",
			}
			json.NewEncoder(os.Stdout).Encode(metadata)
		},
	}

	// Main delta command
	deltaCmd := &cobra.Command{
		Use:   "delta BASE_IMAGE TARGET_IMAGE",
		Short: "Create a binary delta between two images",
		Long: `Create a binary delta between two images using librsync.

The delta is stored as an OCI image with special labels:
  - io.resin.delta.base: The base image ID
  - io.resin.delta.config: JSON metadata about the delta

Examples:
  docker delta busybox:1.35.0 busybox:1.37.0 --tag my-delta:latest
  docker delta my-app:v1 my-app:v2 -t my-app:delta-v1-v2
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

			fmt.Printf("\n✓ Delta image created successfully: %s\n", tag)
			return nil
		},
	}

	deltaCmd.Flags().StringVarP(&tag, "tag", "t", "", "Tag for the delta image (required)")
	deltaCmd.MarkFlagRequired("tag")

	rootCmd.AddCommand(metadataCmd)
	rootCmd.AddCommand(deltaCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
