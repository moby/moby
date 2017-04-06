package formatter

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/command"
)

const (
	defaultNodeTableFormat = "table {{.ID}} {{if .Self}}*{{else}} {{ end }}\t{{.Hostname}}\t{{.Status}}\t{{.Availability}}\t{{.ManagerStatus}}"

	nodeIDHeader        = "ID"
	selfHeader          = ""
	hostnameHeader      = "HOSTNAME"
	availabilityHeader  = "AVAILABILITY"
	managerStatusHeader = "MANAGER STATUS"
)

// NewNodeFormat returns a Format for rendering using a node Context
func NewNodeFormat(source string, quiet bool) Format {
	switch source {
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
