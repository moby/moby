package registry

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type loginOptions struct {
	serverAddress string
	user          string
	password      string
	email         string
}

// NewLoginCommand creates a new `docker login` command
func NewLoginCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts loginOptions

	cmd := &cobra.Command{
		Use:   "login [OPTIONS] [SERVER]",
		Short: "Log in to a Docker registry.",
		Long:  "Log in to a Docker registry.\nIf no server is specified, the default is defined by the daemon.",
		Args:  cli.RequiresMaxArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.serverAddress = args[0]
			}
			return runLogin(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(&opts.user, "username", "u", "", "Username")
	flags.StringVarP(&opts.password, "password", "p", "", "Password")

	// Deprecated in 1.11: Should be removed in docker 1.13
	flags.StringVarP(&opts.email, "email", "e", "", "Email")
	flags.MarkDeprecated("email", "will be removed in 1.13.")

	return cmd
}

func runLogin(dockerCli *client.DockerCli, opts loginOptions) error {
	ctx := context.Background()
	clnt := dockerCli.Client()

	var serverAddress string
	var isDefaultRegistry bool

	// when the user entered  custom registry  url, but the url is the default docker.io registry  so just ignore it
	// without this check the login will succed, but the credentials will not be saved properly and can't be used
	if opts.serverAddress != "" && !strings.Contains(opts.serverAddress, "docker.io") {
		serverAddress = opts.serverAddress
	} else {
		serverAddress = dockerCli.ElectAuthServer(ctx)
		isDefaultRegistry = true
	}
	authConfig, err := dockerCli.ConfigureAuth(opts.user, opts.password, serverAddress, isDefaultRegistry)
	if err != nil {
		return err
	}
	response, err := clnt.RegistryLogin(ctx, authConfig)
	if err != nil {
		return err
	}
	if response.IdentityToken != "" {
		authConfig.Password = ""
		authConfig.IdentityToken = response.IdentityToken
	}
	if err := client.StoreCredentials(dockerCli.ConfigFile(), authConfig); err != nil {
		return fmt.Errorf("Error saving credentials: %v", err)
	}

	if response.Status != "" {
		fmt.Fprintln(dockerCli.Out(), response.Status)
	}
	return nil
}
