package container

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/spf13/cobra"
)

type statsOptions struct {
	all        bool
	noStream   bool
	format     string
	containers []string
}

// NewStatsCommand creates a new cobra.Command for `docker stats`
func NewStatsCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts statsOptions

	cmd := &cobra.Command{
		Use:   "stats [OPTIONS] [CONTAINER...]",
		Short: "Display a live stream of container(s) resource usage statistics",
		Args:  cli.RequiresMinArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			return runStats(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.all, "all", "a", false, "Show all containers (default shows just running)")
	flags.BoolVar(&opts.noStream, "no-stream", false, "Disable streaming stats and only pull the first result")
	flags.StringVar(&opts.format, "format", "", "Pretty-print images using a Go template")
	return cmd
}

// runStats displays a live stream of resource usage statistics for one or more containers.
// This shows real-time information on CPU usage, memory usage, and network I/O.
func runStats(dockerCli *command.DockerCli, opts *statsOptions) error {
	showAll := len(opts.containers) == 0
	closeChan := make(chan error)

	ctx := context.Background()

	// Get the daemonOSType if not set already
	if daemonOSType == "" {
		svctx := context.Background()
		sv, err := dockerCli.Client().ServerVersion(svctx)
		if err != nil {
			return err
		}
		daemonOSType = sv.Os
	}

	// waitFirst is a WaitGroup to wait first stat data's reach for each container
	waitFirst := &sync.WaitGroup{}

	cStats := stats{}
	if showAll {
		waitFirst.Add(1)
		go func() {
			closeChan <- cStats.collectAll(ctx, dockerCli.Client(), opts.all, !opts.noStream, waitFirst)
		}()
	} else {
		// Artificially send creation events for the containers we were asked to
		// monitor (same code path than we use when monitoring all containers).
		for _, name := range opts.containers {
			s := formatter.NewContainerStats(name, daemonOSType)
			if cStats.add(s) {
				waitFirst.Add(1)
				go collect(ctx, s, dockerCli.Client(), !opts.noStream, waitFirst)
			}
		}

		// We don't expect any asynchronous errors: closeChan can be closed.
		close(closeChan)

		// Do a quick pause to detect any error with the provided list of
		// container names.
		time.Sleep(1500 * time.Millisecond)
		var errs []string
		cStats.mu.Lock()
		for _, c := range cStats.cs {
			cErr := c.GetError()
			if cErr != nil && cErr != errNotRunning {
				errs = append(errs, cErr.Error())
			}
		}
		cStats.mu.Unlock()
		if len(errs) > 0 {
			return errors.New(strings.Join(errs, "\n"))
		}
	}

	// before print to screen, make sure each container get at least one valid stat data
	waitFirst.Wait()
	format := opts.format
	if len(format) == 0 {
		if len(dockerCli.ConfigFile().StatsFormat) > 0 {
			format = dockerCli.ConfigFile().StatsFormat
		} else {
			format = formatter.TableFormatKey
		}
	}
	statsCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: formatter.NewStatsFormat(format, daemonOSType),
	}
	cleanScreen := func() {
		if !opts.noStream {
			fmt.Fprint(dockerCli.Out(), "\033[2J")
			fmt.Fprint(dockerCli.Out(), "\033[H")
		}
	}

	var err error
	for range time.Tick(500 * time.Millisecond) {
		cleanScreen()
		ccstats := []formatter.StatsEntry{}
		cStats.mu.Lock()
		for _, c := range cStats.cs {
			ccstats = append(ccstats, c.GetStatistics())
		}
		cStats.mu.Unlock()
		if err = formatter.ContainerStatsWrite(statsCtx, ccstats); err != nil {
			break
		}
		if len(cStats.cs) == 0 && !showAll {
			break
		}
		if opts.noStream {
			break
		}
		select {
		case err, ok := <-closeChan:
			if ok {
				if err != nil {
					// this is suppressing "unexpected EOF" in the cli when the
					// daemon restarts so it shutdowns cleanly
					if err == io.ErrUnexpectedEOF {
						return nil
					}
					return err
				}
			}
		default:
			// just skip
		}
	}
	return err
}
