package image

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/registry"
	"github.com/spf13/cobra"
)

type pullOptions struct {
	remote string
	all    bool
}

// NewPullCommand creates a new `docker pull` command
func NewPullCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts pullOptions

	cmd := &cobra.Command{
		Use:   "pull [OPTIONS] NAME[:TAG|@DIGEST]",
		Short: "Pull an image or a repository from a registry",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.remote = args[0]
			return runPull(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(&opts.all, "all-tags", "a", false, "Download all tagged images in the repository")
	command.AddTrustVerificationFlags(flags)

	return cmd
}

func runPull(dockerCli *command.DockerCli, opts pullOptions) error {
	var distributionRef reference.Named
	distributionRef, err := reference.ParseNormalizedNamed(opts.remote)
	if err != nil {
		return err
	}
	if opts.all && !reference.IsNameOnly(distributionRef) {
		return errors.New("tag can't be used with --all-tags/-a")
	}

	if !opts.all && reference.IsNameOnly(distributionRef) {
		taggedRef := reference.EnsureTagged(distributionRef)
		fmt.Fprintf(dockerCli.Out(), "Using default tag: %s\n", taggedRef.Tag())
		distributionRef = taggedRef
	}

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(distributionRef)
	if err != nil {
		return err
	}

	ctx := context.Background()

	authConfig := command.ResolveAuthConfig(ctx, dockerCli, repoInfo.Index)
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(dockerCli, repoInfo.Index, "pull")

	// Check if reference has a digest
	_, isCanonical := distributionRef.(reference.Canonical)
	if command.IsTrusted() && !isCanonical {
		err = trustedPull(ctx, dockerCli, repoInfo, distributionRef, authConfig, requestPrivilege)
	} else {
		err = imagePullPrivileged(ctx, dockerCli, authConfig, reference.FamiliarString(distributionRef), requestPrivilege, opts.all)
	}
	if err != nil {
		if strings.Contains(err.Error(), "target is plugin") {
			return errors.New(err.Error() + " - Use `docker plugin install`")
		}
		return err
	}

	return nil
}
