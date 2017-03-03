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
	networkCtx := networkContext{}
	networkCtx.header = networkHeaderContext{
		"ID":        networkIDHeader,
		"Name":      nameHeader,
		"Driver":    driverHeader,
		"Scope":     scopeHeader,
		"IPv6":      ipv6Header,
		"Internal":  internalHeader,
		"Labels":    labelsHeader,
		"CreatedAt": createdAtHeader,
	}
	return ctx.Write(&networkCtx, render)
}

type networkHeaderContext map[string]string

func (c networkHeaderContext) Label(name string) string {
	n := strings.Split(name, ".")
	r := strings.NewReplacer("-", " ", "_", " ")
	h := r.Replace(n[len(n)-1])

	return h
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
	if c.trunc {
		return stringid.TruncateID(c.n.ID)
	}
	return c.n.ID
}

func (c *networkContext) Name() string {
	return c.n.Name
}

func (c *networkContext) Driver() string {
	return c.n.Driver
}

func (c *networkContext) Scope() string {
	return c.n.Scope
}

func (c *networkContext) IPv6() string {
	return fmt.Sprintf("%v", c.n.EnableIPv6)
}

func (c *networkContext) Internal() string {
	return fmt.Sprintf("%v", c.n.Internal)
}

func (c *networkContext) Labels() string {
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
	if c.n.Labels == nil {
		return ""
	}
	return c.n.Labels[name]
}

func (c *networkContext) CreatedAt() string {
	return c.n.Created.String()
}
