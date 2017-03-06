package formatter

import (
	"bytes"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestDiffContextFormatWrite(t *testing.T) {
	// Check default output format (verbose and non-verbose mode) for table headers
	cases := []struct {
		context  Context
		expected string
	}{
		{
			Context{Format: NewDiffFormat("table")},
			`CHANGE TYPE         PATH
C                   /var/log/app.log
A                   /usr/app/app.js
D                   /usr/app/old_app.js
`,
		},
		{
			Context{Format: NewDiffFormat("table {{.Path}}")},
			`PATH
/var/log/app.log
/usr/app/app.js
/usr/app/old_app.js
`,
		},
		{
			Context{Format: NewDiffFormat("{{.Type}}: {{.Path}}")},
			`C: /var/log/app.log
A: /usr/app/app.js
D: /usr/app/old_app.js
`,
		},
	}

	diffs := []container.ContainerChangeResponseItem{
		{archive.ChangeModify, "/var/log/app.log"},
		{archive.ChangeAdd, "/usr/app/app.js"},
		{archive.ChangeDelete, "/usr/app/old_app.js"},
	}

	for _, testcase := range cases {
		out := bytes.NewBufferString("")
		testcase.context.Output = out
		err := DiffWrite(testcase.context, diffs)
		if err != nil {
			assert.Error(t, err, testcase.expected)
		} else {
			assert.Equal(t, out.String(), testcase.expected)
		}
	}
}
