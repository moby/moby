package formatter

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/inspect"
	units "github.com/docker/go-units"
)

const (
	defaultNodeTableFormat           = "table {{.ID}} {{if .Self}}*{{else}} {{ end }}\t{{.Hostname}}\t{{.Status}}\t{{.Availability}}\t{{.ManagerStatus}}"
	nodeInspectPrettyTemplate Format = `ID:			{{.ID}}
{{- if .Name }}
Name:			{{.Name}}
{{- end }}
{{- if .Labels }}
Labels:
{{- range $k, $v := .Labels }}
 - {{ $k }}{{if $v }}={{ $v }}{{ end }}
{{- end }}{{ end }}
Hostname:              	{{.Hostname}}
Joined at:             	{{.CreatedAt}}
Status:
 State:			{{.StatusState}}
 {{- if .HasStatusMessage}}
 Message:              	{{.StatusMessage}}
 {{- end}}
 Availability:         	{{.SpecAvailability}}
 {{- if .Status.Addr}}
 Address:		{{.StatusAddr}}
 {{- end}}
{{- if .HasManagerStatus}}
Manager Status:
 Address:		{{.ManagerStatusAddr}}
 Raft Status:		{{.ManagerStatusReachability}}
 {{- if .IsManagerStatusLeader}}
 Leader:		Yes
 {{- else}}
 Leader:		No
 {{- end}}
{{- end}}
Platform:
 Operating System:	{{.PlatformOS}}
 Architecture:		{{.PlatformArchitecture}}
Resources:
 CPUs:			{{.ResourceNanoCPUs}}
 Memory:		{{.ResourceMemory}}
{{- if .HasEnginePlugins}}
Plugins:
{{- range $k, $v := .EnginePlugins }}
 {{ $k }}:{{if $v }}		{{ $v }}{{ end }}
{{- end }}
{{- end }}
Engine Version:		{{.EngineVersion}}
{{- if .EngineLabels}}
Engine Labels:
{{- range $k, $v := .EngineLabels }}
 - {{ $k }}{{if $v }}={{ $v }}{{ end }}
{{- end }}{{- end }}
`
	nodeIDHeader        = "ID"
	selfHeader          = ""
	hostnameHeader      = "HOSTNAME"
	availabilityHeader  = "AVAILABILITY"
	managerStatusHeader = "MANAGER STATUS"
)

// NewNodeFormat returns a Format for rendering using a node Context
func NewNodeFormat(source string, quiet bool) Format {
	switch source {
	case PrettyFormatKey:
		return nodeInspectPrettyTemplate
	case TableFormatKey:
		if quiet {
			return defaultQuietFormat
		}
		return defaultNodeTableFormat
	case RawFormatKey:
		if quiet {
			return `node_id: {{.ID}}`
		}
		return `node_id: {{.ID}}\nhostname: {{.Hostname}}\nstatus: {{.Status}}\navailability: {{.Availability}}\nmanager_status: {{.ManagerStatus}}\n`
	}
	return Format(source)
}

// NodeWrite writes the context
func NodeWrite(ctx Context, nodes []swarm.Node, info types.Info) error {
	render := func(format func(subContext subContext) error) error {
		for _, node := range nodes {
			nodeCtx := &nodeContext{n: node, info: info}
			if err := format(nodeCtx); err != nil {
				return err
			}
		}
		return nil
	}
	nodeCtx := nodeContext{}
	nodeCtx.header = nodeHeaderContext{
		"ID":            nodeIDHeader,
		"Self":          selfHeader,
		"Hostname":      hostnameHeader,
		"Status":        statusHeader,
		"Availability":  availabilityHeader,
		"ManagerStatus": managerStatusHeader,
	}
	return ctx.Write(&nodeCtx, render)
}

type nodeHeaderContext map[string]string

type nodeContext struct {
	HeaderContext
	n    swarm.Node
	info types.Info
}

func (c *nodeContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(c)
}

func (c *nodeContext) ID() string {
	return c.n.ID
}

func (c *nodeContext) Self() bool {
	return c.n.ID == c.info.Swarm.NodeID
}

func (c *nodeContext) Hostname() string {
	return c.n.Description.Hostname
}

func (c *nodeContext) Status() string {
	return command.PrettyPrint(string(c.n.Status.State))
}

func (c *nodeContext) Availability() string {
	return command.PrettyPrint(string(c.n.Spec.Availability))
}

func (c *nodeContext) ManagerStatus() string {
	reachability := ""
	if c.n.ManagerStatus != nil {
		if c.n.ManagerStatus.Leader {
			reachability = "Leader"
		} else {
			reachability = string(c.n.ManagerStatus.Reachability)
		}
	}
	return command.PrettyPrint(reachability)
}

// NodeInspectWrite renders the context for a list of services
func NodeInspectWrite(ctx Context, refs []string, getRef inspect.GetRefFunc) error {
	if ctx.Format != nodeInspectPrettyTemplate {
		return inspect.Inspect(ctx.Output, refs, string(ctx.Format), getRef)
	}
	render := func(format func(subContext subContext) error) error {
		for _, ref := range refs {
			nodeI, _, err := getRef(ref)
			if err != nil {
				return err
			}
			node, ok := nodeI.(swarm.Node)
			if !ok {
				return fmt.Errorf("got wrong object to inspect :%v", ok)
			}
			if err := format(&nodeInspectContext{Node: node}); err != nil {
				return err
			}
		}
		return nil
	}
	return ctx.Write(&nodeInspectContext{}, render)
}

type nodeInspectContext struct {
	swarm.Node
	subContext
}

func (ctx *nodeInspectContext) ID() string {
	return ctx.Node.ID
}

func (ctx *nodeInspectContext) Name() string {
	return ctx.Node.Spec.Name
}

func (ctx *nodeInspectContext) Labels() map[string]string {
	return ctx.Node.Spec.Labels
}

func (ctx *nodeInspectContext) Hostname() string {
	return ctx.Node.Description.Hostname
}

func (ctx *nodeInspectContext) CreatedAt() string {
	return command.PrettyPrint(ctx.Node.CreatedAt)
}

func (ctx *nodeInspectContext) StatusState() string {
	return command.PrettyPrint(ctx.Node.Status.State)
}

func (ctx *nodeInspectContext) HasStatusMessage() bool {
	return ctx.Node.Status.Message != ""
}

func (ctx *nodeInspectContext) StatusMessage() string {
	return command.PrettyPrint(ctx.Node.Status.Message)
}

func (ctx *nodeInspectContext) SpecAvailability() string {
	return command.PrettyPrint(ctx.Node.Spec.Availability)
}

func (ctx *nodeInspectContext) HasStatusAddr() bool {
	return ctx.Node.Status.Addr != ""
}

func (ctx *nodeInspectContext) StatusAddr() string {
	return ctx.Node.Status.Addr
}

func (ctx *nodeInspectContext) HasManagerStatus() bool {
	return ctx.Node.ManagerStatus != nil
}

func (ctx *nodeInspectContext) ManagerStatusAddr() string {
	return ctx.Node.ManagerStatus.Addr
}

func (ctx *nodeInspectContext) ManagerStatusReachability() string {
	return command.PrettyPrint(ctx.Node.ManagerStatus.Reachability)
}

func (ctx *nodeInspectContext) IsManagerStatusLeader() bool {
	return ctx.Node.ManagerStatus.Leader
}

func (ctx *nodeInspectContext) PlatformOS() string {
	return ctx.Node.Description.Platform.OS
}

func (ctx *nodeInspectContext) PlatformArchitecture() string {
	return ctx.Node.Description.Platform.Architecture
}

func (ctx *nodeInspectContext) ResourceNanoCPUs() int {
	if ctx.Node.Description.Resources.NanoCPUs == 0 {
		return int(0)
	}
	return int(ctx.Node.Description.Resources.NanoCPUs) / 1e9
}

func (ctx *nodeInspectContext) ResourceMemory() string {
	if ctx.Node.Description.Resources.MemoryBytes == 0 {
		return ""
	}
	return units.BytesSize(float64(ctx.Node.Description.Resources.MemoryBytes))
}

func (ctx *nodeInspectContext) HasEnginePlugins() bool {
	return len(ctx.Node.Description.Engine.Plugins) > 0
}

func (ctx *nodeInspectContext) EnginePlugins() map[string]string {
	pluginMap := map[string][]string{}
	for _, p := range ctx.Node.Description.Engine.Plugins {
		pluginMap[p.Type] = append(pluginMap[p.Type], p.Name)
	}

	pluginNamesByType := map[string]string{}
	for k, v := range pluginMap {
		pluginNamesByType[k] = strings.Join(v, ", ")
	}
	return pluginNamesByType
}

func (ctx *nodeInspectContext) EngineLabels() map[string]string {
	return ctx.Node.Description.Engine.Labels
}

func (ctx *nodeInspectContext) EngineVersion() string {
	return ctx.Node.Description.Engine.EngineVersion
}
