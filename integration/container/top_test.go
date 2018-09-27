package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"testing"

	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/test/request"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/skip"
)

func TestContainerTop(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows")
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	id := container.Run(t, ctx, client, container.WithCmd("sleep", "100000"))

	cases := []struct {
		opts      []string
		expectErr bool
		numProc   int
	}{
		{
			opts:      []string{""},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"-C", "sleep"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"-o", "pid,cmd"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"-opid,cmd"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"opid,cmd"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"eopid,cmd"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"-Csleep", "eopid,cmd,cgroup", "--sort=pid"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"o pid,cmd"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"--format", "pid,cmd"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"-o", "pid,cmd", "oargs,uid", "--format", "%cpu"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"-C", "sleep", "-o", "cmd"},
			expectErr: true,
		},
		{
			opts:      []string{"-C", "sleep", "-o", "pid,cmd"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"-Csleep", "-opid,cmd"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"-o uid=PID,cmd"},
			expectErr: true,
		},
		{
			opts:      []string{"axf"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"--no-headers"},
			expectErr: true,
		},
		{
			opts:      []string{"aux", "--sort=comm"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"aux", "--sort", "pid"},
			expectErr: false,
			numProc:   1,
		},
		{
			opts:      []string{"aux", "--sort", "pid", "--noheader"},
			expectErr: true,
		},
		{
			opts:      []string{"-o"},
			expectErr: true,
		},
		{
			opts:      []string{"-o", "sasha"},
			expectErr: true,
		},
	}

	for _, c := range cases {
		resp, err := client.ContainerTop(ctx, id, c.opts)
		if !c.expectErr {
			t.Logf("req: %v; response: %+v", c.opts, resp)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(len(resp.Processes), c.numProc))
		} else {
			t.Logf("req: %v; err: %v", c.opts, err)
			assert.Check(t, err != nil)
		}
	}
}
