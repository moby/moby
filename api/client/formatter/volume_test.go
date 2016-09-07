package formatter

import (
	"bytes"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringid"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestVolumeContext(c *check.C) {
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

	for _, ca := range cases {
		ctx = ca.volumeCtx
		v := ca.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(c, v, ca.expValue)
		} else if v != ca.expValue {
			c.Fatalf("Expected %s, was %s\n", ca.expValue, v)
		}

		c.Assert(ctx.fullHeader(), check.Equals, ca.expHeader)
	}
}

func (s *DockerSuite) TestVolumeContextWrite(c *check.C) {
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
		c.Assert(out.String(), check.Equals, context.expected)
		// Clean buffer
		out.Reset()
	}
}
