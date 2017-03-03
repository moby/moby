package formatter

import (
	"fmt"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/go-units"
)

const (
	defaultTaskTableFormat = "table {{.ID}}\t{{.Name}}\t{{.Image}}\t{{.Node}}\t{{.DesiredState}}\t{{.CurrentState}}\t{{.Error}}\t{{.Ports}}"

	nodeHeader         = "NODE"
	taskIDHeader       = "ID"
	desiredStateHeader = "DESIRED STATE"
	currentStateHeader = "CURRENT STATE"
	errorHeader        = "ERROR"

	maxErrLength = 30
)

// NewTaskFormat returns a Format for rendering using a task Context
func NewTaskFormat(source string, quiet bool) Format {
	switch source {
	case TableFormatKey:
		if quiet {
			return defaultQuietFormat
		}
		return defaultTaskTableFormat
	case RawFormatKey:
		if quiet {
			return `id: {{.ID}}`
		}
		return `id: {{.ID}}\nname: {{.Name}}\nimage: {{.Image}}\nnode: {{.Node}}\ndesired_state: {{.DesiredState}}\ncurrent_state: {{.CurrentState}}\nerror: {{.Error}}\nports: {{.Ports}}\n`
	}
	return Format(source)
}

// TaskWrite writes the context
func TaskWrite(ctx Context, tasks []swarm.Task, names map[string]string, nodes map[string]string) error {
	render := func(format func(subContext subContext) error) error {
		for _, task := range tasks {
			taskCtx := &taskContext{trunc: ctx.Trunc, task: task, name: names[task.ID], node: nodes[task.ID]}
			if err := format(taskCtx); err != nil {
				return err
			}
		}
		return nil
	}
	taskCtx := taskContext{}
	taskCtx.header = taskHeaderContext{
		"ID":           taskIDHeader,
		"Name":         nameHeader,
		"Image":        imageHeader,
		"Node":         nodeHeader,
		"DesiredState": desiredStateHeader,
		"CurrentState": currentStateHeader,
		"Error":        errorHeader,
		"Ports":        portsHeader,
	}
	return ctx.Write(&taskCtx, render)
}

type taskHeaderContext map[string]string

type taskContext struct {
	HeaderContext
	trunc bool
	task  swarm.Task
	name  string
	node  string
}

func (c *taskContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(c)
}

func (c *taskContext) ID() string {
	if c.trunc {
		return stringid.TruncateID(c.task.ID)
	}
	return c.task.ID
}

func (c *taskContext) Name() string {
	return c.name
}

func (c *taskContext) Image() string {
	image := c.task.Spec.ContainerSpec.Image
	if c.trunc {
		ref, err := reference.ParseNormalizedNamed(image)
		if err == nil {
			// update image string for display, (strips any digest)
			if nt, ok := ref.(reference.NamedTagged); ok {
				if namedTagged, err := reference.WithTag(reference.TrimNamed(nt), nt.Tag()); err == nil {
					image = reference.FamiliarString(namedTagged)
				}
			}
		}
	}
	return image
}

func (c *taskContext) Node() string {
	return c.node
}

func (c *taskContext) DesiredState() string {
	return command.PrettyPrint(c.task.DesiredState)
}

func (c *taskContext) CurrentState() string {
	return fmt.Sprintf("%s %s ago",
		command.PrettyPrint(c.task.Status.State),
		strings.ToLower(units.HumanDuration(time.Since(c.task.Status.Timestamp))),
	)
}

func (c *taskContext) Error() string {
	// Trim and quote the error message.
	taskErr := c.task.Status.Err
	if c.trunc && len(taskErr) > maxErrLength {
		taskErr = fmt.Sprintf("%s…", taskErr[:maxErrLength-1])
	}
	if len(taskErr) > 0 {
		taskErr = fmt.Sprintf("\"%s\"", taskErr)
	}
	return taskErr
}

func (c *taskContext) Ports() string {
	if len(c.task.Status.PortStatus.Ports) == 0 {
		return ""
	}
	ports := []string{}
	for _, pConfig := range c.task.Status.PortStatus.Ports {
		ports = append(ports, fmt.Sprintf("*:%d->%d/%s",
			pConfig.PublishedPort,
			pConfig.TargetPort,
			pConfig.Protocol,
		))
	}
	return strings.Join(ports, ",")
}
