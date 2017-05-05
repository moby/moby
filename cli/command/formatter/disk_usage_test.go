package formatter

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiskUsageContextFormatWrite(t *testing.T) {
	cases := []struct {
		context  DiskUsageContext
		expected string
	}{
		// Check default output format (verbose and non-verbose mode) for table headers
		{
			DiskUsageContext{
				Context: Context{
					Format: NewDiskUsageFormat("table"),
				},
				Verbose: false},
			`TYPE                TOTAL               ACTIVE              SIZE                RECLAIMABLE
Images              0                   0                   0B                  0B
Containers          0                   0                   0B                  0B
Local Volumes       0                   0                   0B                  0B
`,
		},
		{
			DiskUsageContext{Verbose: true},
			`Images space usage:

REPOSITORY          TAG                 IMAGE ID            CREATED ago         SIZE                SHARED SIZE         UNIQUE SiZE         CONTAINERS

Containers space usage:

CONTAINER ID        IMAGE               COMMAND             LOCAL VOLUMES       SIZE                CREATED ago         STATUS              NAMES

Local Volumes space usage:

VOLUME NAME         LINKS               SIZE
`,
		},
		// Errors
		{
			DiskUsageContext{
				Context: Context{
					Format: "{{InvalidFunction}}",
				},
			},
			`Template parsing error: template: :1: function "InvalidFunction" not defined
`,
		},
		{
			DiskUsageContext{
				Context: Context{
					Format: "{{nil}}",
				},
			},
			`Template parsing error: template: :1:2: executing "" at <nil>: nil is not a command
`,
		},
		// Table Format
		{
			DiskUsageContext{
				Context: Context{
					Format: NewDiskUsageFormat("table"),
				},
			},
			`TYPE                TOTAL               ACTIVE              SIZE                RECLAIMABLE
Images              0                   0                   0B                  0B
Containers          0                   0                   0B                  0B
Local Volumes       0                   0                   0B                  0B
`,
		},
		{
			DiskUsageContext{
				Context: Context{
					Format: NewDiskUsageFormat("table {{.Type}}\t{{.Active}}"),
				},
			},
			`TYPE                ACTIVE
Images              0
Containers          0
Local Volumes       0
`,
		},
		// Raw Format
		{
			DiskUsageContext{
				Context: Context{
					Format: NewDiskUsageFormat("raw"),
				},
			},
			`type: Images
total: 0
active: 0
size: 0B
reclaimable: 0B

type: Containers
total: 0
active: 0
size: 0B
reclaimable: 0B

type: Local Volumes
total: 0
active: 0
size: 0B
reclaimable: 0B

`,
		},
	}

	for _, testcase := range cases {
		out := bytes.NewBufferString("")
		testcase.context.Output = out
		if err := testcase.context.Write(); err != nil {
			assert.Equal(t, testcase.expected, err.Error())
		} else {
			assert.Equal(t, testcase.expected, out.String())
		}
	}
}
