package prune

import (
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/container"
	"github.com/docker/docker/cli/command/image"
	"github.com/docker/docker/cli/command/network"
	"github.com/docker/docker/cli/command/volume"
	"github.com/spf13/cobra"
)

// NewContainerPruneCommand returns a cobra prune command for containers
func NewContainerPruneCommand(dockerCli *command.DockerCli) *cobra.Command {
	return container.NewPruneCommand(dockerCli)
}

// NewVolumePruneCommand returns a cobra prune command for volumes
func NewVolumePruneCommand(dockerCli *command.DockerCli) *cobra.Command {
	return volume.NewPruneCommand(dockerCli)
}

// NewImagePruneCommand returns a cobra prune command for images
func NewImagePruneCommand(dockerCli *command.DockerCli) *cobra.Command {
	return image.NewPruneCommand(dockerCli)
}

// NewNetworkPruneCommand returns a cobra prune command for Networks
func NewNetworkPruneCommand(dockerCli *command.DockerCli) *cobra.Command {
	return network.NewPruneCommand(dockerCli)
}

// RunContainerPrune executes a prune command for containers
func RunContainerPrune(dockerCli *command.DockerCli) (uint64, string, error) {
	return container.RunPrune(dockerCli)
}

// RunVolumePrune executes a prune command for volumes
func RunVolumePrune(dockerCli *command.DockerCli) (uint64, string, error) {
	return volume.RunPrune(dockerCli)
}

// RunImagePrune executes a prune command for images
func RunImagePrune(dockerCli *command.DockerCli, all bool) (uint64, string, error) {
	return image.RunPrune(dockerCli, all)
}

// RunNetworkPrune executes a prune command for networks
func RunNetworkPrune(dockerCli *command.DockerCli) (uint64, string, error) {
	return network.RunPrune(dockerCli)
}
