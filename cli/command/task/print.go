package task

import (
	"fmt"
	"sort"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/cli/command/idresolver"
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

// Print task information in a format.
// Besides this, command `docker node ps <node>`
// and `docker stack ps` will call this, too.
func Print(dockerCli command.Cli, ctx context.Context, tasks []swarm.Task, resolver *idresolver.IDResolver, trunc, quiet bool, format string) error {
	sort.Stable(tasksBySlot(tasks))

	names := map[string]string{}
	nodes := map[string]string{}

	tasksCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: formatter.NewTaskFormat(format, quiet),
		Trunc:  trunc,
	}

	prevName := ""
	for _, task := range tasks {
		serviceName, err := resolver.Resolve(ctx, swarm.Service{}, task.ServiceID)
		if err != nil {
			return err
		}

		nodeValue, err := resolver.Resolve(ctx, swarm.Node{}, task.NodeID)
		if err != nil {
			return err
		}

		name := ""
		if task.Slot != 0 {
			name = fmt.Sprintf("%v.%v", serviceName, task.Slot)
		} else {
			name = fmt.Sprintf("%v.%v", serviceName, task.NodeID)
		}

		// Indent the name if necessary
		indentedName := name
		if name == prevName {
			indentedName = fmt.Sprintf(" \\_ %s", indentedName)
		}
		prevName = name

		names[task.ID] = name
		if tasksCtx.Format.IsTable() {
			names[task.ID] = indentedName
		}
		nodes[task.ID] = nodeValue
	}

	return formatter.TaskWrite(tasksCtx, tasks, names, nodes)
}
