package command

import (
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/container"
	"github.com/docker/docker/api/client/image"
	"github.com/docker/docker/api/client/network"
	"github.com/docker/docker/api/client/node"
	"github.com/docker/docker/api/client/plugin"
	"github.com/docker/docker/api/client/registry"
	"github.com/docker/docker/api/client/service"
	"github.com/docker/docker/api/client/stack"
	"github.com/docker/docker/api/client/swarm"
	"github.com/docker/docker/api/client/system"
	"github.com/docker/docker/api/client/volume"
	"github.com/spf13/cobra"
)

// AddCommands adds all the commands from api/client to the root command
func AddCommands(cmd *cobra.Command, dockerCli *client.DockerCli) {
	cmd.AddCommand(
		node.NewNodeCommand(dockerCli),
		service.NewServiceCommand(dockerCli),
		stack.NewStackCommand(dockerCli),
		stack.NewTopLevelDeployCommand(dockerCli),
		swarm.NewSwarmCommand(dockerCli),
		container.NewAttachCommand(dockerCli),
		container.NewCommitCommand(dockerCli),
		container.NewCopyCommand(dockerCli),
		container.NewCreateCommand(dockerCli),
		container.NewDiffCommand(dockerCli),
		container.NewExecCommand(dockerCli),
		container.NewExportCommand(dockerCli),
		container.NewKillCommand(dockerCli),
		container.NewLogsCommand(dockerCli),
		container.NewPauseCommand(dockerCli),
		container.NewPortCommand(dockerCli),
		container.NewPsCommand(dockerCli),
		container.NewRenameCommand(dockerCli),
		container.NewRestartCommand(dockerCli),
		container.NewRmCommand(dockerCli),
		container.NewRunCommand(dockerCli),
		container.NewStartCommand(dockerCli),
		container.NewStatsCommand(dockerCli),
		container.NewStopCommand(dockerCli),
		container.NewTopCommand(dockerCli),
		container.NewUnpauseCommand(dockerCli),
		container.NewUpdateCommand(dockerCli),
		container.NewWaitCommand(dockerCli),
		image.NewBuildCommand(dockerCli),
		image.NewHistoryCommand(dockerCli),
		image.NewImagesCommand(dockerCli),
		image.NewLoadCommand(dockerCli),
		image.NewRemoveCommand(dockerCli),
		image.NewSaveCommand(dockerCli),
		image.NewPullCommand(dockerCli),
		image.NewPushCommand(dockerCli),
		image.NewSearchCommand(dockerCli),
		image.NewImportCommand(dockerCli),
		image.NewTagCommand(dockerCli),
		network.NewNetworkCommand(dockerCli),
		system.NewEventsCommand(dockerCli),
		system.NewInspectCommand(dockerCli),
		registry.NewLoginCommand(dockerCli),
		registry.NewLogoutCommand(dockerCli),
		system.NewVersionCommand(dockerCli),
		volume.NewVolumeCommand(dockerCli),
		system.NewInfoCommand(dockerCli),
	)
	plugin.NewPluginCommand(cmd, dockerCli)
}
