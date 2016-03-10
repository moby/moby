package client

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/events"
	"github.com/docker/engine-api/types/filters"
)

// CmdStats displays a live stream of resource usage statistics for one or more containers.
//
// This shows real-time information on CPU usage, memory usage, and network I/O.
//
// Usage: docker stats [OPTIONS] [CONTAINER...]
func (cli *DockerCli) CmdStats(args ...string) error {
	cmd := Cli.Subcmd("stats", []string{"[CONTAINER...]"}, Cli.DockerCommands["stats"].Description, true)
	all := cmd.Bool([]string{"a", "-all"}, false, "Show all containers (default shows just running)")
	noStream := cmd.Bool([]string{"-no-stream"}, false, "Disable streaming stats and only pull the first result")

	cmd.ParseFlags(args, true)

	names := cmd.Args()
	showAll := len(names) == 0
	closeChan := make(chan error)

	// monitorContainerEvents watches for container creation and removal (only
	// used when calling `docker stats` without arguments).
	monitorContainerEvents := func(started chan<- struct{}, c chan events.Message) {
		f := filters.NewArgs()
		f.Add("type", "container")
		options := types.EventsOptions{
			Filters: f,
		}
		resBody, err := cli.client.Events(context.Background(), options)
		// Whether we successfully subscribed to events or not, we can now
		// unblock the main goroutine.
		close(started)
		if err != nil {
			closeChan <- err
			return
		}
		defer resBody.Close()

		decodeEvents(resBody, func(event events.Message, err error) error {
			if err != nil {
				closeChan <- err
				return nil
			}
			c <- event
			return nil
		})
	}

	// waitFirst is a WaitGroup to wait first stat data's reach for each container
	waitFirst := &sync.WaitGroup{}

	cStats := stats{}
	// getContainerList simulates creation event for all previously existing
	// containers (only used when calling `docker stats` without arguments).
	getContainerList := func() {
		options := types.ContainerListOptions{
			All: *all,
		}
		cs, err := cli.client.ContainerList(options)
		if err != nil {
			closeChan <- err
		}
		for _, container := range cs {
			s := &containerStats{Name: container.ID[:12]}
			if cStats.add(s) {
				waitFirst.Add(1)
				go s.Collect(cli.client, !*noStream, waitFirst)
			}
		}
	}

	if showAll {
		// If no names were specified, start a long running goroutine which
		// monitors container events. We make sure we're subscribed before
		// retrieving the list of running containers to avoid a race where we
		// would "miss" a creation.
		started := make(chan struct{})
		eh := eventHandler{handlers: make(map[string]func(events.Message))}
		eh.Handle("create", func(e events.Message) {
			if *all {
				s := &containerStats{Name: e.ID[:12]}
				if cStats.add(s) {
					waitFirst.Add(1)
					go s.Collect(cli.client, !*noStream, waitFirst)
				}
			}
		})

		eh.Handle("start", func(e events.Message) {
			s := &containerStats{Name: e.ID[:12]}
			if cStats.add(s) {
				waitFirst.Add(1)
				go s.Collect(cli.client, !*noStream, waitFirst)
			}
		})

		eh.Handle("die", func(e events.Message) {
			if !*all {
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
		for _, name := range names {
			s := &containerStats{Name: name}
			if cStats.add(s) {
				waitFirst.Add(1)
				go s.Collect(cli.client, !*noStream, waitFirst)
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
			c.mu.Lock()
			if c.err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", c.Name, c.err))
			}
			c.mu.Unlock()
		}
		cStats.mu.Unlock()
		if len(errs) > 0 {
			return fmt.Errorf("%s", strings.Join(errs, ", "))
		}
	}

	// before print to screen, make sure each container get at least one valid stat data
	waitFirst.Wait()

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	printHeader := func() {
		if !*noStream {
			fmt.Fprint(cli.out, "\033[2J")
			fmt.Fprint(cli.out, "\033[H")
		}
		io.WriteString(w, "CONTAINER\tCPU %\tMEM USAGE / LIMIT\tMEM %\tNET I/O\tBLOCK I/O\tPIDS\n")
	}

	for range time.Tick(500 * time.Millisecond) {
		printHeader()
		toRemove := []int{}
		cStats.mu.Lock()
		for i, s := range cStats.cs {
			if err := s.Display(w); err != nil && !*noStream {
				toRemove = append(toRemove, i)
			}
		}
		for j := len(toRemove) - 1; j >= 0; j-- {
			i := toRemove[j]
			cStats.cs = append(cStats.cs[:i], cStats.cs[i+1:]...)
		}
		if len(cStats.cs) == 0 && !showAll {
			return nil
		}
		cStats.mu.Unlock()
		w.Flush()
		if *noStream {
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
	return nil
}
