package formatter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/engine-api/types"
)

func TestNetworkContext(t *testing.T) {
	networkID := stringid.GenerateRandomID()

	var ctx networkContext
	cases := []struct {
		networkCtx networkContext
		expValue   string
		expHeader  string
		call       func() string
	}{
		{networkContext{
			n:     types.NetworkResource{ID: networkID},
			trunc: false,
		}, networkID, networkIDHeader, ctx.ID},
		{networkContext{
			n:     types.NetworkResource{ID: networkID},
			trunc: true,
		}, stringid.TruncateID(networkID), networkIDHeader, ctx.ID},
		{networkContext{
			n: types.NetworkResource{Name: "network_name"},
		}, "network_name", nameHeader, ctx.Name},
		{networkContext{
			n: types.NetworkResource{Driver: "driver_name"},
		}, "driver_name", driverHeader, ctx.Driver},
		{networkContext{
			n: types.NetworkResource{EnableIPv6: true},
		}, "true", ipv6Header, ctx.IPv6},
		{networkContext{
			n: types.NetworkResource{EnableIPv6: false},
		}, "false", ipv6Header, ctx.IPv6},
		{networkContext{
			n: types.NetworkResource{Internal: true},
		}, "true", internalHeader, ctx.Internal},
		{networkContext{
			n: types.NetworkResource{Internal: false},
		}, "false", internalHeader, ctx.Internal},
		{networkContext{
			n: types.NetworkResource{},
		}, "", labelsHeader, ctx.Labels},
		{networkContext{
			n: types.NetworkResource{Labels: map[string]string{"label1": "value1", "label2": "value2"}},
		}, "label1=value1,label2=value2", labelsHeader, ctx.Labels},
	}

	for _, c := range cases {
		ctx = c.networkCtx
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}

		h := ctx.fullHeader()
		if h != c.expHeader {
			t.Fatalf("Expected %s, was %s\n", c.expHeader, h)
		}
	}
}

func TestNetworkContextWrite(t *testing.T) {
	contexts := []struct {
		context  NetworkContext
		expected string
	}{

		// Errors
		{
			NetworkContext{
				Context: Context{
					Format: "{{InvalidFunction}}",
				},
			},
			`Template parsing error: template: :1: function "InvalidFunction" not defined
`,
		},
		{
			NetworkContext{
				Context: Context{
					Format: "{{nil}}",
				},
			},
			`Template parsing error: template: :1:2: executing "" at <nil>: nil is not a command
`,
		},
		// Table format
		{
			NetworkContext{
				Context: Context{
					Format: "table",
				},
			},
			`NETWORK ID          NAME                DRIVER              SCOPE
networkID1          foobar_baz          foo                 local
networkID2          foobar_bar          bar                 local
`,
		},
		{
			NetworkContext{
				Context: Context{
					Format: "table",
					Quiet:  true,
				},
			},
			`networkID1
networkID2
`,
		},
		{
			NetworkContext{
				Context: Context{
					Format: "table {{.Name}}",
				},
			},
			`NAME
foobar_baz
foobar_bar
`,
		},
		{
			NetworkContext{
				Context: Context{
					Format: "table {{.Name}}",
					Quiet:  true,
				},
			},
			`NAME
foobar_baz
foobar_bar
`,
		},
		// Raw Format
		{
			NetworkContext{
				Context: Context{
					Format: "raw",
				},
			}, `network_id: networkID1
name: foobar_baz
driver: foo
scope: local

network_id: networkID2
name: foobar_bar
driver: bar
scope: local

`,
		},
		{
			NetworkContext{
				Context: Context{
					Format: "raw",
					Quiet:  true,
				},
			},
			`network_id: networkID1
network_id: networkID2
`,
		},
		// Custom Format
		{
			NetworkContext{
				Context: Context{
					Format: "{{.Name}}",
				},
			},
			`foobar_baz
foobar_bar
`,
		},
	}

	for _, context := range contexts {
		networks := []types.NetworkResource{
			{ID: "networkID1", Name: "foobar_baz", Driver: "foo", Scope: "local"},
			{ID: "networkID2", Name: "foobar_bar", Driver: "bar", Scope: "local"},
		}
		out := bytes.NewBufferString("")
		context.context.Output = out
		context.context.Networks = networks
		context.context.Write()
		actual := out.String()
		if actual != context.expected {
			t.Fatalf("Expected \n%s, got \n%s", context.expected, actual)
		}
		// Clean buffer
		out.Reset()
	}
}
