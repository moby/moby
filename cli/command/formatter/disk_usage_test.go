package formatter

import (
	"bytes"
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
)

func TestDiskUsageContextFormatWrite(t *testing.T) {
	// Check default output format (verbose and non-verbose mode) for table headers
	cases := []struct {
		context  DiskUsageContext
		expected string
	}{
		{
			DiskUsageContext{Verbose: false},
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
	}

	for _, testcase := range cases {
		out := bytes.NewBufferString("")
		testcase.context.Output = out
		testcase.context.Write()
		assert.Equal(t, out.String(), testcase.expected)
	}
}
