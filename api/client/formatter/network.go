package formatter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/engine-api/types"
)

const (
	defaultNetworkTableFormat = "table {{.ID}}\t{{.Name}}\t{{.Driver}}\t{{.Scope}}"

	networkIDHeader = "NETWORK ID"
	ipv6Header      = "IPV6"
	internalHeader  = "INTERNAL"
)

// NetworkContext contains network specific information required by the formatter,
// encapsulate a Context struct.
type NetworkContext struct {
	Context
	// Networks
	Networks []types.NetworkResource
}

func (ctx NetworkContext) Write() {
	switch ctx.Format {
	case tableFormatKey:
		if ctx.Quiet {
			ctx.Format = defaultQuietFormat
		} else {
			ctx.Format = defaultNetworkTableFormat
		}
	case rawFormatKey:
		if ctx.Quiet {
			ctx.Format = `network_id: {{.ID}}`
		} else {
			ctx.Format = `network_id: {{.ID}}\nname: {{.Name}}\ndriver: {{.Driver}}\nscope: {{.Scope}}\n`
		}
	}

	ctx.buffer = bytes.NewBufferString("")
	ctx.preformat()

	tmpl, err := ctx.parseFormat()
	if err != nil {
		return
	}

	for _, network := range ctx.Networks {
		networkCtx := &networkContext{
			trunc: ctx.Trunc,
			n:     network,
		}
		err = ctx.contextFormat(tmpl, networkCtx)
		if err != nil {
			return
		}
	}

	ctx.postformat(tmpl, &networkContext{})
}

type networkContext struct {
	baseSubContext
	trunc bool
	n     types.NetworkResource
}

func (c *networkContext) ID() string {
	c.addHeader(networkIDHeader)
	if c.trunc {
		return stringid.TruncateID(c.n.ID)
	}
	return c.n.ID
}

func (c *networkContext) Name() string {
	c.addHeader(nameHeader)
	return c.n.Name
}

func (c *networkContext) Driver() string {
	c.addHeader(driverHeader)
	return c.n.Driver
}

func (c *networkContext) Scope() string {
	c.addHeader(scopeHeader)
	return c.n.Scope
}

func (c *networkContext) IPv6() string {
	c.addHeader(ipv6Header)
	return fmt.Sprintf("%v", c.n.EnableIPv6)
}

func (c *networkContext) Internal() string {
	c.addHeader(internalHeader)
	return fmt.Sprintf("%v", c.n.Internal)
}

func (c *networkContext) Labels() string {
	c.addHeader(labelsHeader)
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

	c.addHeader(h)

	if c.n.Labels == nil {
		return ""
	}
	return c.n.Labels[name]
}
