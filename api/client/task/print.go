package task

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/idresolver"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/go-units"
)

const (
	psTaskItemFmt = "%s\t%s\t%s\t%s\t%s %s\t%s\t%s\n"
)

type tasksByInstance []swarm.Task

func (t tasksByInstance) Len() int {
	return len(t)
}

func (t tasksByInstance) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t tasksByInstance) Less(i, j int) bool {
	// Sort by instance.
	if t[i].Instance != t[j].Instance {
		return t[i].Instance < t[j].Instance
	}

	// If same instance, sort by most recent.
	return t[j].Meta.CreatedAt.Before(t[i].CreatedAt)
}

// Print task information in a table format
func Print(dockerCli *client.DockerCli, ctx context.Context, tasks []swarm.Task, resolver *idresolver.IDResolver) error {
	sort.Stable(tasksByInstance(tasks))

	writer := tabwriter.NewWriter(dockerCli.Out(), 0, 4, 2, ' ', 0)

	// Ignore flushing errors
	defer writer.Flush()
	fmt.Fprintln(writer, strings.Join([]string{"ID", "NAME", "SERVICE", "IMAGE", "LAST STATE", "DESIRED STATE", "NODE"}, "\t"))
	for _, task := range tasks {
		serviceValue, err := resolver.Resolve(ctx, swarm.Service{}, task.ServiceID)
		if err != nil {
			return err
		}
		nodeValue, err := resolver.Resolve(ctx, swarm.Node{}, task.NodeID)
		if err != nil {
			return err
		}
		name := serviceValue
		if task.Instance > 0 {
			name = fmt.Sprintf("%s.%d", name, task.Instance)
		}
		fmt.Fprintf(
			writer,
			psTaskItemFmt,
			task.ID,
			name,
			serviceValue,
			task.Spec.ContainerSpec.Image,
			task.Status.State, units.HumanDuration(time.Since(task.Status.Timestamp)),
			task.DesiredState,
			nodeValue,
		)
	}

	return nil
}
