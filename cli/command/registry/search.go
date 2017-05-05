package registry

import (
	"sort"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/registry"
	"github.com/spf13/cobra"
)

type searchOptions struct {
	term    string
	noTrunc bool
	limit   int
	filter  opts.FilterOpt
	format  string

	// Deprecated
	stars     uint
	automated bool
}

// NewSearchCommand creates a new `docker search` command
func NewSearchCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := searchOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "search [OPTIONS] TERM",
		Short: "Search the Docker Hub for images",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.term = args[0]
			return runSearch(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "Don't truncate output")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")
	flags.IntVar(&opts.limit, "limit", registry.DefaultSearchLimit, "Max number of search results")
	flags.StringVar(&opts.format, "format", "", "Pretty-print search using a Go template")

	flags.BoolVar(&opts.automated, "automated", false, "Only show automated builds")
	flags.UintVarP(&opts.stars, "stars", "s", 0, "Only displays with at least x stars")

	flags.MarkDeprecated("automated", "use --filter=is-automated=true instead")
	flags.MarkDeprecated("stars", "use --filter=stars=3 instead")

	return cmd
}

func runSearch(dockerCli *command.DockerCli, opts searchOptions) error {
	indexInfo, err := registry.ParseSearchIndexInfo(opts.term)
	if err != nil {
		return err
	}

	ctx := context.Background()

	authConfig := command.ResolveAuthConfig(ctx, dockerCli, indexInfo)
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(dockerCli, indexInfo, "search")

	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}

	options := types.ImageSearchOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
		Filters:       opts.filter.Value(),
		Limit:         opts.limit,
	}

	clnt := dockerCli.Client()

	unorderedResults, err := clnt.ImageSearch(ctx, opts.term, options)
	if err != nil {
		return err
	}

	results := searchResultsByStars(unorderedResults)
	sort.Sort(results)

	format := opts.format
	if len(format) == 0 {
		format = formatter.TableFormatKey
	}

	searchCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: formatter.NewSearchFormat(format),
		Trunc:  !opts.noTrunc,
	}

	return formatter.SearchWrite(searchCtx, results, opts.automated, int(opts.stars))
}

// searchResultsByStars sorts search results in descending order by number of stars.
type searchResultsByStars []registrytypes.SearchResult

func (r searchResultsByStars) Len() int           { return len(r) }
func (r searchResultsByStars) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r searchResultsByStars) Less(i, j int) bool { return r[j].StarCount < r[i].StarCount }
