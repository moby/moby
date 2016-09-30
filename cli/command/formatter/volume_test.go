package formatter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestVolumeContext(t *testing.T) {
	volumeName := stringid.GenerateRandomID()

	var ctx volumeContext
	cases := []struct {
		volumeCtx volumeContext
		expValue  string
		expHeader string
		call      func() string
	}{
		{volumeContext{
			v: types.Volume{Name: volumeName},
		}, volumeName, nameHeader, ctx.Name},
		{volumeContext{
			v: types.Volume{Driver: "driver_name"},
		}, "driver_name", driverHeader, ctx.Driver},
		{volumeContext{
			v: types.Volume{Scope: "local"},
		}, "local", scopeHeader, ctx.Scope},
		{volumeContext{
			v: types.Volume{Mountpoint: "mountpoint"},
		}, "mountpoint", mountpointHeader, ctx.Mountpoint},
		{volumeContext{
			v: types.Volume{},
		}, "", labelsHeader, ctx.Labels},
		{volumeContext{
			v: types.Volume{Labels: map[string]string{"label1": "value1", "label2": "value2"}},
		}, "label1=value1,label2=value2", labelsHeader, ctx.Labels},
	}

	for _, c := range cases {
		ctx = c.volumeCtx
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

func TestVolumeContextWrite(t *testing.T) {
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
			Context{Format: NewVolumeFormat("table", false)},
			`DRIVER              NAME
foo                 foobar_baz
bar                 foobar_bar
`,
		},
		{
			Context{Format: NewVolumeFormat("table", true)},
			`foobar_baz
foobar_bar
`,
		},
		{
			Context{Format: NewVolumeFormat("table {{.Name}}", false)},
			`NAME
foobar_baz
foobar_bar
`,
		},
		{
			Context{Format: NewVolumeFormat("table {{.Name}}", true)},
			`NAME
foobar_baz
foobar_bar
`,
		},
		// Raw Format
		{
			Context{Format: NewVolumeFormat("raw", false)},
			`name: foobar_baz
driver: foo

name: foobar_bar
driver: bar

`,
		},
		{
			Context{Format: NewVolumeFormat("raw", true)},
			`name: foobar_baz
name: foobar_bar
`,
		},
		// Custom Format
		{
			Context{Format: NewVolumeFormat("{{.Name}}", false)},
			`foobar_baz
foobar_bar
`,
		},
	}

	for _, testcase := range cases {
		volumes := []*types.Volume{
			{Name: "foobar_baz", Driver: "foo"},
			{Name: "foobar_bar", Driver: "bar"},
		}
		out := bytes.NewBufferString("")
		testcase.context.Output = out
		err := VolumeWrite(testcase.context, volumes)
		if err != nil {
			assert.Error(t, err, testcase.expected)
		} else {
			assert.Equal(t, out.String(), testcase.expected)
		}
	}
}
