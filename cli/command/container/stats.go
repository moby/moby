package container

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
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

	// monitorContainerEvents watches for container creation and removal (only
	// used when calling `docker stats` without arguments).
	monitorContainerEvents := func(started chan<- struct{}, c chan events.Message) {
		f := filters.NewArgs()
		f.Add("type", "container")
		options := types.EventsOptions{
			Filters: f,
		}

		eventq, errq := dockerCli.Client().Events(ctx, options)

		// Whether we successfully subscribed to eventq or not, we can now
		// unblock the main goroutine.
		close(started)

		for {
			select {
			case event := <-eventq:
				c <- event
			case err := <-errq:
				closeChan <- err
				return
			}
		}
	}

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
	// getContainerList simulates creation event for all previously existing
	// containers (only used when calling `docker stats` without arguments).
	getContainerList := func() {
		options := types.ContainerListOptions{
			All: opts.all,
		}
		cs, err := dockerCli.Client().ContainerList(ctx, options)
		if err != nil {
			closeChan <- err
		}
		for _, container := range cs {
			s := formatter.NewContainerStats(container.ID[:12], daemonOSType)
			if cStats.add(s) {
				waitFirst.Add(1)
				go collect(ctx, s, dockerCli.Client(), !opts.noStream, waitFirst)
			}
		}
	}

	if showAll {
		// If no names were specified, start a long running goroutine which
		// monitors container events. We make sure we're subscribed before
		// retrieving the list of running containers to avoid a race where we
		// would "miss" a creation.
		started := make(chan struct{})
		eh := command.InitEventHandler()
		eh.Handle("create", func(e events.Message) {
			if opts.all {
				s := formatter.NewContainerStats(e.ID[:12], daemonOSType)
				if cStats.add(s) {
					waitFirst.Add(1)
					go collect(ctx, s, dockerCli.Client(), !opts.noStream, waitFirst)
				}
			}
		})

		eh.Handle("start", func(e events.Message) {
			s := formatter.NewContainerStats(e.ID[:12], daemonOSType)
			if cStats.add(s) {
				waitFirst.Add(1)
				go collect(ctx, s, dockerCli.Client(), !opts.noStream, waitFirst)
			}
		})

		eh.Handle("die", func(e events.Message) {
			if !opts.all {
				cStats.remove(e.ID[:12])
			}
		})

		eventChan := make(chan events.Message)
		go eh.Watch(eventChan)
		go monitorContainerEvents(started, eventChan)
		defer close(eventChan)
		<-started

		// Start a short-lived goroutine to retrieve the initial list of
		// containers.
		getContainerList()
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
			if err := c.GetError(); err != nil {
				errs = append(errs, err.Error())
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
		if err = formatter.ContainerStatsWrite(statsCtx, ccstats, daemonOSType); err != nil {
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
