package formatter

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringid"
)

const (
	defaultNetworkTableFormat = "table {{.ID}}\t{{.Name}}\t{{.Driver}}\t{{.Scope}}"

	networkIDHeader = "NETWORK ID"
	ipv6Header      = "IPV6"
	internalHeader  = "INTERNAL"
)

// NewNetworkFormat returns a Format for rendering using a network Context
func NewNetworkFormat(source string, quiet bool) Format {
	switch source {
	case TableFormatKey:
		if quiet {
			return defaultQuietFormat
		}
		return defaultNetworkTableFormat
	case RawFormatKey:
		if quiet {
			return `network_id: {{.ID}}`
		}
		return `network_id: {{.ID}}\nname: {{.Name}}\ndriver: {{.Driver}}\nscope: {{.Scope}}\n`
	}
	return Format(source)
}

// NetworkWrite writes the context
func NetworkWrite(ctx Context, networks []types.NetworkResource) error {
	render := func(format func(subContext subContext) error) error {
		for _, network := range networks {
			networkCtx := &networkContext{trunc: ctx.Trunc, n: network}
			if err := format(networkCtx); err != nil {
				return err
			}
		}
		return nil
	}
	return ctx.Write(&networkContext{}, render)
}

type networkContext struct {
	HeaderContext
	trunc bool
	n     types.NetworkResource
}

func (c *networkContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(c)
}

func (c *networkContext) ID() string {
	c.AddHeader(networkIDHeader)
	if c.trunc {
		return stringid.TruncateID(c.n.ID)
	}
	return c.n.ID
}

func (c *networkContext) Name() string {
	c.AddHeader(nameHeader)
	return c.n.Name
}

func (c *networkContext) Driver() string {
	c.AddHeader(driverHeader)
	return c.n.Driver
}

func (c *networkContext) Scope() string {
	c.AddHeader(scopeHeader)
	return c.n.Scope
}

func (c *networkContext) IPv6() string {
	c.AddHeader(ipv6Header)
	return fmt.Sprintf("%v", c.n.EnableIPv6)
}

func (c *networkContext) Internal() string {
	c.AddHeader(internalHeader)
	return fmt.Sprintf("%v", c.n.Internal)
}

func (c *networkContext) Labels() string {
	c.AddHeader(labelsHeader)
	if c.n.Labels == nil {
		return ""
	}

	var joinLabels []string
	for k, v := range c.n.Labels {
		joinLabels = append(joinLabels, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(joinLabels, ",")
}

func (c *networkContext) Label(name string) string {
	n := strings.Split(name, ".")
	r := strings.NewReplacer("-", " ", "_", " ")
	h := r.Replace(n[len(n)-1])

	c.AddHeader(h)

	if c.n.Labels == nil {
		return ""
	}
	return c.n.Labels[name]
}

func (c *networkContext) CreatedAt() string {
	c.AddHeader(createdAtHeader)
	return c.n.Created.String()
}
