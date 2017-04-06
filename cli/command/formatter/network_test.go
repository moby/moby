package formatter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

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
		call       func() string
	}{
		{networkContext{
			n:     types.NetworkResource{ID: networkID},
			trunc: false,
		}, networkID, ctx.ID},
		{networkContext{
			n:     types.NetworkResource{ID: networkID},
			trunc: true,
		}, stringid.TruncateID(networkID), ctx.ID},
		{networkContext{
			n: types.NetworkResource{Name: "network_name"},
		}, "network_name", ctx.Name},
		{networkContext{
			n: types.NetworkResource{Driver: "driver_name"},
		}, "driver_name", ctx.Driver},
		{networkContext{
			n: types.NetworkResource{EnableIPv6: true},
		}, "true", ctx.IPv6},
		{networkContext{
			n: types.NetworkResource{EnableIPv6: false},
		}, "false", ctx.IPv6},
		{networkContext{
			n: types.NetworkResource{Internal: true},
		}, "true", ctx.Internal},
		{networkContext{
			n: types.NetworkResource{Internal: false},
		}, "false", ctx.Internal},
		{networkContext{
			n: types.NetworkResource{},
		}, "", ctx.Labels},
		{networkContext{
			n: types.NetworkResource{Labels: map[string]string{"label1": "value1", "label2": "value2"}},
		}, "label1=value1,label2=value2", ctx.Labels},
	}

	for _, c := range cases {
		ctx = c.networkCtx
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
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
		// Custom Format with CreatedAt
		{
			Context{Format: NewNetworkFormat("{{.Name}} {{.CreatedAt}}", false)},
			`foobar_baz 2016-01-01 00:00:00 +0000 UTC
foobar_bar 2017-01-01 00:00:00 +0000 UTC
`,
		},
	}

	timestamp1, _ := time.Parse("2006-01-02", "2016-01-01")
	timestamp2, _ := time.Parse("2006-01-02", "2017-01-01")

	for _, testcase := range cases {
		networks := []types.NetworkResource{
			{ID: "networkID1", Name: "foobar_baz", Driver: "foo", Scope: "local", Created: timestamp1},
			{ID: "networkID2", Name: "foobar_bar", Driver: "bar", Scope: "local", Created: timestamp2},
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

func TestNetworkContextWriteJSON(t *testing.T) {
	networks := []types.NetworkResource{
		{ID: "networkID1", Name: "foobar_baz"},
		{ID: "networkID2", Name: "foobar_bar"},
	}
	expectedJSONs := []map[string]interface{}{
		{"Driver": "", "ID": "networkID1", "IPv6": "false", "Internal": "false", "Labels": "", "Name": "foobar_baz", "Scope": "", "CreatedAt": "0001-01-01 00:00:00 +0000 UTC"},
		{"Driver": "", "ID": "networkID2", "IPv6": "false", "Internal": "false", "Labels": "", "Name": "foobar_bar", "Scope": "", "CreatedAt": "0001-01-01 00:00:00 +0000 UTC"},
	}

	out := bytes.NewBufferString("")
	err := NetworkWrite(Context{Format: "{{json .}}", Output: out}, networks)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		t.Logf("Output: line %d: %s", i, line)
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatal(err)
		}
		assert.DeepEqual(t, m, expectedJSONs[i])
	}
}

func TestNetworkContextWriteJSONField(t *testing.T) {
	networks := []types.NetworkResource{
		{ID: "networkID1", Name: "foobar_baz"},
		{ID: "networkID2", Name: "foobar_bar"},
	}
	out := bytes.NewBufferString("")
	err := NetworkWrite(Context{Format: "{{json .ID}}", Output: out}, networks)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		t.Logf("Output: line %d: %s", i, line)
		var s string
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, s, networks[i].ID)
	}
}
