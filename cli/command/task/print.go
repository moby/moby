package task

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/idresolver"
	"github.com/docker/go-units"
)

const (
	psTaskItemFmt = "%s\t%s\t%s\t%s\t%s %s ago\t%s\n"
	maxErrLength  = 30
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
func Print(dockerCli *command.DockerCli, ctx context.Context, tasks []swarm.Task, resolver *idresolver.IDResolver, noTrunc bool) error {
	sort.Stable(tasksBySlot(tasks))

	writer := tabwriter.NewWriter(dockerCli.Out(), 0, 4, 2, ' ', 0)

	// Ignore flushing errors
	defer writer.Flush()
	fmt.Fprintln(writer, strings.Join([]string{"NAME", "IMAGE", "NODE", "DESIRED STATE", "CURRENT STATE", "ERROR"}, "\t"))

	prevServiceName := ""
	prevSlot := 0
	for _, task := range tasks {
		serviceName, err := resolver.Resolve(ctx, swarm.Service{}, task.ServiceID)
		if err != nil {
			return err
		}
		nodeValue, err := resolver.Resolve(ctx, swarm.Node{}, task.NodeID)
		if err != nil {
			return err
		}

		name := task.Annotations.Name
		// TODO: This is the fallback <ServiceName>.<Slot>.<taskID> in case task name is not present in
		// Annotations (upgraded from 1.12).
		// We may be able to remove the following in the future.
		if name == "" {
			if task.Slot != 0 {
				name = fmt.Sprintf("%v.%v.%v", serviceName, task.Slot, task.ID)
			} else {
				name = fmt.Sprintf("%v.%v.%v", serviceName, task.NodeID, task.ID)
			}
		}

		// Indent the name if necessary
		indentedName := name
		// Since the new format of the task name is <ServiceName>.<Slot>.<taskID>, we should only compare
		// <ServiceName> and <Slot> here.
		if prevServiceName == serviceName && prevSlot == task.Slot {
			indentedName = fmt.Sprintf(" \\_ %s", indentedName)
		}
		prevServiceName = serviceName
		prevSlot = task.Slot

		// Trim and quote the error message.
		taskErr := task.Status.Err
		if !noTrunc && len(taskErr) > maxErrLength {
			taskErr = fmt.Sprintf("%s…", taskErr[:maxErrLength-1])
		}
		if len(taskErr) > 0 {
			taskErr = fmt.Sprintf("\"%s\"", taskErr)
		}

		fmt.Fprintf(
			writer,
			psTaskItemFmt,
			indentedName,
			task.Spec.ContainerSpec.Image,
			nodeValue,
			command.PrettyPrint(task.DesiredState),
			command.PrettyPrint(task.Status.State),
			strings.ToLower(units.HumanDuration(time.Since(task.Status.Timestamp))),
			taskErr,
		)
	}

	return nil
}
