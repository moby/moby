package formatter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/engine-api/types"
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
			v: &types.Volume{Name: volumeName},
		}, volumeName, nameHeader, ctx.Name},
		{volumeContext{
			v: &types.Volume{Driver: "driver_name"},
		}, "driver_name", driverHeader, ctx.Driver},
		{volumeContext{
			v: &types.Volume{Scope: "local"},
		}, "local", scopeHeader, ctx.Scope},
		{volumeContext{
			v: &types.Volume{Mountpoint: "mountpoint"},
		}, "mountpoint", mountpointHeader, ctx.Mountpoint},
		{volumeContext{
			v: &types.Volume{},
		}, "", labelsHeader, ctx.Labels},
		{volumeContext{
			v: &types.Volume{Labels: map[string]string{"label1": "value1", "label2": "value2"}},
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

		h := ctx.fullHeader()
		if h != c.expHeader {
			t.Fatalf("Expected %s, was %s\n", c.expHeader, h)
		}
	}
}

func TestVolumeContextWrite(t *testing.T) {
	contexts := []struct {
		context  VolumeContext
		expected string
	}{

		// Errors
		{
			VolumeContext{
				Context: Context{
					Format: "{{InvalidFunction}}",
				},
			},
			`Template parsing error: template: :1: function "InvalidFunction" not defined
`,
		},
		{
			VolumeContext{
				Context: Context{
					Format: "{{nil}}",
				},
			},
			`Template parsing error: template: :1:2: executing "" at <nil>: nil is not a command
`,
		},
		// Table format
		{
			VolumeContext{
				Context: Context{
					Format: "table",
				},
			},
			`DRIVER              NAME
foo                 foobar_baz
bar                 foobar_bar
`,
		},
		{
			VolumeContext{
				Context: Context{
					Format: "table",
					Quiet:  true,
				},
			},
			`foobar_baz
foobar_bar
`,
		},
		{
			VolumeContext{
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
			VolumeContext{
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
			VolumeContext{
				Context: Context{
					Format: "raw",
				},
			}, `name: foobar_baz
driver: foo

name: foobar_bar
driver: bar

`,
		},
		{
			VolumeContext{
				Context: Context{
					Format: "raw",
					Quiet:  true,
				},
			},
			`name: foobar_baz
name: foobar_bar
`,
		},
		// Custom Format
		{
			VolumeContext{
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
		volumes := []*types.Volume{
			{Name: "foobar_baz", Driver: "foo"},
			{Name: "foobar_bar", Driver: "bar"},
		}
		out := bytes.NewBufferString("")
		context.context.Output = out
		context.context.Volumes = volumes
		context.context.Write()
		actual := out.String()
		if actual != context.expected {
			t.Fatalf("Expected \n%s, got \n%s", context.expected, actual)
		}
		// Clean buffer
		out.Reset()
	}
}
