package formatter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/stringid"
	"github.com/stretchr/testify/assert"
)

func TestNodeContext(t *testing.T) {
	nodeID := stringid.GenerateRandomID()

	var ctx nodeContext
	cases := []struct {
		nodeCtx  nodeContext
		expValue string
		call     func() string
	}{
		{nodeContext{
			n: swarm.Node{ID: nodeID},
		}, nodeID, ctx.ID},
		{nodeContext{
			n: swarm.Node{Description: swarm.NodeDescription{Hostname: "node_hostname"}},
		}, "node_hostname", ctx.Hostname},
		{nodeContext{
			n: swarm.Node{Status: swarm.NodeStatus{State: swarm.NodeState("foo")}},
		}, "Foo", ctx.Status},
		{nodeContext{
			n: swarm.Node{Spec: swarm.NodeSpec{Availability: swarm.NodeAvailability("drain")}},
		}, "Drain", ctx.Availability},
		{nodeContext{
			n: swarm.Node{ManagerStatus: &swarm.ManagerStatus{Leader: true}},
		}, "Leader", ctx.ManagerStatus},
	}

	for _, c := range cases {
		ctx = c.nodeCtx
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}
	}
}

func TestNodeContextWrite(t *testing.T) {
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
			Context{Format: NewNodeFormat("table", false)},
			`ID                  HOSTNAME            STATUS              AVAILABILITY        MANAGER STATUS
nodeID1             foobar_baz          Foo                 Drain               Leader
nodeID2             foobar_bar          Bar                 Active              Reachable
`,
		},
		{
			Context{Format: NewNodeFormat("table", true)},
			`nodeID1
nodeID2
`,
		},
		{
			Context{Format: NewNodeFormat("table {{.Hostname}}", false)},
			`HOSTNAME
foobar_baz
foobar_bar
`,
		},
		{
			Context{Format: NewNodeFormat("table {{.Hostname}}", true)},
			`HOSTNAME
foobar_baz
foobar_bar
`,
		},
		// Raw Format
		{
			Context{Format: NewNodeFormat("raw", false)},
			`node_id: nodeID1
hostname: foobar_baz
status: Foo
availability: Drain
manager_status: Leader

node_id: nodeID2
hostname: foobar_bar
status: Bar
availability: Active
manager_status: Reachable

`,
		},
		{
			Context{Format: NewNodeFormat("raw", true)},
			`node_id: nodeID1
node_id: nodeID2
`,
		},
		// Custom Format
		{
			Context{Format: NewNodeFormat("{{.Hostname}}", false)},
			`foobar_baz
foobar_bar
`,
		},
	}

	for _, testcase := range cases {
		nodes := []swarm.Node{
			{ID: "nodeID1", Description: swarm.NodeDescription{Hostname: "foobar_baz"}, Status: swarm.NodeStatus{State: swarm.NodeState("foo")}, Spec: swarm.NodeSpec{Availability: swarm.NodeAvailability("drain")}, ManagerStatus: &swarm.ManagerStatus{Leader: true}},
			{ID: "nodeID2", Description: swarm.NodeDescription{Hostname: "foobar_bar"}, Status: swarm.NodeStatus{State: swarm.NodeState("bar")}, Spec: swarm.NodeSpec{Availability: swarm.NodeAvailability("active")}, ManagerStatus: &swarm.ManagerStatus{Leader: false, Reachability: swarm.Reachability("Reachable")}},
		}
		out := bytes.NewBufferString("")
		testcase.context.Output = out
		err := NodeWrite(testcase.context, nodes, types.Info{})
		if err != nil {
			assert.EqualError(t, err, testcase.expected)
		} else {
			assert.Equal(t, testcase.expected, out.String())
		}
	}
}

func TestNodeContextWriteJSON(t *testing.T) {
	nodes := []swarm.Node{
		{ID: "nodeID1", Description: swarm.NodeDescription{Hostname: "foobar_baz"}},
		{ID: "nodeID2", Description: swarm.NodeDescription{Hostname: "foobar_bar"}},
	}
	expectedJSONs := []map[string]interface{}{
		{"Availability": "", "Hostname": "foobar_baz", "ID": "nodeID1", "ManagerStatus": "", "Status": "", "Self": false},
		{"Availability": "", "Hostname": "foobar_bar", "ID": "nodeID2", "ManagerStatus": "", "Status": "", "Self": false},
	}

	out := bytes.NewBufferString("")
	err := NodeWrite(Context{Format: "{{json .}}", Output: out}, nodes, types.Info{})
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		t.Logf("Output: line %d: %s", i, line)
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectedJSONs[i], m)
	}
}

func TestNodeContextWriteJSONField(t *testing.T) {
	nodes := []swarm.Node{
		{ID: "nodeID1", Description: swarm.NodeDescription{Hostname: "foobar_baz"}},
		{ID: "nodeID2", Description: swarm.NodeDescription{Hostname: "foobar_bar"}},
	}
	out := bytes.NewBufferString("")
	err := NodeWrite(Context{Format: "{{json .ID}}", Output: out}, nodes, types.Info{})
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		t.Logf("Output: line %d: %s", i, line)
		var s string
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, nodes[i].ID, s)
	}
}
