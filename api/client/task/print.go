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
	psTaskItemFmt = "%s\t%s\t%s\t%s\t%s %s ago\t%s\t%s\n"
)

type tasksBySlot []swarm.Task

func (t tasksBySlot) Len() int {
	return len(t)
}

func (t tasksBySlot) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t tasksBySlot) Less(i, j int) bool {
	// Sort by slot.
	if t[i].Slot != t[j].Slot {
		return t[i].Slot < t[j].Slot
	}

	// If same slot, sort by most recent.
	return t[j].Meta.CreatedAt.Before(t[i].CreatedAt)
}

// Print task information in a table format
func Print(dockerCli *client.DockerCli, ctx context.Context, tasks []swarm.Task, resolver *idresolver.IDResolver) error {
	sort.Stable(tasksBySlot(tasks))

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
		if task.Slot > 0 {
			name = fmt.Sprintf("%s.%d", name, task.Slot)
		}
		fmt.Fprintf(
			writer,
			psTaskItemFmt,
			task.ID,
			name,
			serviceValue,
			task.Spec.ContainerSpec.Image,
			client.PrettyPrint(task.Status.State),
			strings.ToLower(units.HumanDuration(time.Since(task.Status.Timestamp))),
			client.PrettyPrint(task.DesiredState),
			nodeValue,
		)
	}

	return nil
}
