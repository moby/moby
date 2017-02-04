package formatter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestPluginContext(t *testing.T) {
	pluginID := stringid.GenerateRandomID()

	var ctx pluginContext
	cases := []struct {
		pluginCtx pluginContext
		expValue  string
		call      func() string
	}{
		{pluginContext{
			p:     types.Plugin{ID: pluginID},
			trunc: false,
		}, pluginID, ctx.ID},
		{pluginContext{
			p:     types.Plugin{ID: pluginID},
			trunc: true,
		}, stringid.TruncateID(pluginID), ctx.ID},
		{pluginContext{
			p: types.Plugin{Name: "plugin_name"},
		}, "plugin_name", ctx.Name},
		{pluginContext{
			p: types.Plugin{Config: types.PluginConfig{Description: "plugin_description"}},
		}, "plugin_description", ctx.Description},
	}

	for _, c := range cases {
		ctx = c.pluginCtx
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}
	}
}

func TestPluginContextWrite(t *testing.T) {
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
			Context{Format: NewPluginFormat("table", false)},
			`ID                  NAME                DESCRIPTION         ENABLED
pluginID1           foobar_baz          description 1       true
pluginID2           foobar_bar          description 2       false
`,
		},
		{
			Context{Format: NewPluginFormat("table", true)},
			`pluginID1
pluginID2
`,
		},
		{
			Context{Format: NewPluginFormat("table {{.Name}}", false)},
			`NAME
foobar_baz
foobar_bar
`,
		},
		{
			Context{Format: NewPluginFormat("table {{.Name}}", true)},
			`NAME
foobar_baz
foobar_bar
`,
		},
		// Raw Format
		{
			Context{Format: NewPluginFormat("raw", false)},
			`plugin_id: pluginID1
name: foobar_baz
description: description 1
enabled: true

plugin_id: pluginID2
name: foobar_bar
description: description 2
enabled: false

`,
		},
		{
			Context{Format: NewPluginFormat("raw", true)},
			`plugin_id: pluginID1
plugin_id: pluginID2
`,
		},
		// Custom Format
		{
			Context{Format: NewPluginFormat("{{.Name}}", false)},
			`foobar_baz
foobar_bar
`,
		},
	}

	for _, testcase := range cases {
		plugins := []*types.Plugin{
			{ID: "pluginID1", Name: "foobar_baz", Config: types.PluginConfig{Description: "description 1"}, Enabled: true},
			{ID: "pluginID2", Name: "foobar_bar", Config: types.PluginConfig{Description: "description 2"}, Enabled: false},
		}
		out := bytes.NewBufferString("")
		testcase.context.Output = out
		err := PluginWrite(testcase.context, plugins)
		if err != nil {
			assert.Error(t, err, testcase.expected)
		} else {
			assert.Equal(t, out.String(), testcase.expected)
		}
	}
}

func TestPluginContextWriteJSON(t *testing.T) {
	plugins := []*types.Plugin{
		{ID: "pluginID1", Name: "foobar_baz"},
		{ID: "pluginID2", Name: "foobar_bar"},
	}
	expectedJSONs := []map[string]interface{}{
		{"Description": "", "Enabled": false, "ID": "pluginID1", "Name": "foobar_baz", "PluginReference": ""},
		{"Description": "", "Enabled": false, "ID": "pluginID2", "Name": "foobar_bar", "PluginReference": ""},
	}

	out := bytes.NewBufferString("")
	err := PluginWrite(Context{Format: "{{json .}}", Output: out}, plugins)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatal(err)
		}
		assert.DeepEqual(t, m, expectedJSONs[i])
	}
}

func TestPluginContextWriteJSONField(t *testing.T) {
	plugins := []*types.Plugin{
		{ID: "pluginID1", Name: "foobar_baz"},
		{ID: "pluginID2", Name: "foobar_bar"},
	}
	out := bytes.NewBufferString("")
	err := PluginWrite(Context{Format: "{{json .ID}}", Output: out}, plugins)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var s string
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, s, plugins[i].ID)
	}
}
