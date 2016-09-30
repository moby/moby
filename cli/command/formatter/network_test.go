package formatter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/testutil/assert"
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

		h := ctx.FullHeader()
		if h != c.expHeader {
			t.Fatalf("Expected %s, was %s\n", c.expHeader, h)
		}
	}
}

func TestNetworkContextWrite(t *testing.T) {
	cases := []struct {
		context  Context
		expected string
	}{

		// Errors
		{
			Context{Format: "{{InvalidFunction}}"},
			`Template parsing error: template: :1: function "InvalidFunction" not defined
`,
		},
		{
			Context{Format: "{{nil}}"},
			`Template parsing error: template: :1:2: executing "" at <nil>: nil is not a command
`,
		},
		// Table format
		{
			Context{Format: NewNetworkFormat("table", false)},
			`NETWORK ID          NAME                DRIVER              SCOPE
networkID1          foobar_baz          foo                 local
networkID2          foobar_bar          bar                 local
`,
		},
		{
			Context{Format: NewNetworkFormat("table", true)},
			`networkID1
networkID2
`,
		},
		{
			Context{Format: NewNetworkFormat("table {{.Name}}", false)},
			`NAME
foobar_baz
foobar_bar
`,
		},
		{
			Context{Format: NewNetworkFormat("table {{.Name}}", true)},
			`NAME
foobar_baz
foobar_bar
`,
		},
		// Raw Format
		{
			Context{Format: NewNetworkFormat("raw", false)},
			`network_id: networkID1
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
			Context{Format: NewNetworkFormat("raw", true)},
			`network_id: networkID1
network_id: networkID2
`,
		},
		// Custom Format
		{
			Context{Format: NewNetworkFormat("{{.Name}}", false)},
			`foobar_baz
foobar_bar
`,
		},
	}

	for _, testcase := range cases {
		networks := []types.NetworkResource{
			{ID: "networkID1", Name: "foobar_baz", Driver: "foo", Scope: "local"},
			{ID: "networkID2", Name: "foobar_bar", Driver: "bar", Scope: "local"},
		}
		out := bytes.NewBufferString("")
		testcase.context.Output = out
		err := NetworkWrite(testcase.context, networks)
		if err != nil {
			assert.Error(t, err, testcase.expected)
		} else {
			assert.Equal(t, out.String(), testcase.expected)
		}
	}
}
