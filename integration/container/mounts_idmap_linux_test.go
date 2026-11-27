package container

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	containertypes "github.com/moby/moby/api/types/container"
	mounttypes "github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/versions"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/pkg/parsers/kernel"
	"github.com/moby/sys/mount"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// TestContainerBindMountIDMapped runs real images with an id-mapped bind
// mount whose source belongs to a host user that none of the images know.
// Inside the container, the source must appear owned by the user that needs
// it.
// Everything the container writes through the mount must land on the host
// owned by the source's owner.
//
// The comment on each case is the equivalent docker CLI invocation, using
// the bind-idmap mount option.
func TestContainerBindMountIDMapped(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "id-mapped mounts require a rootful daemon")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.57"), "requires API v1.57")
	skip.If(t, !kernel.CheckKernelVersion(6, 3, 0), "id-mapped tmpfs mounts require kernel 6.3")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	// Neither an image user nor inside the userns-remap range, so ownership
	// seen in the container can only come from the mapping.
	const hostOwner = 3000

	const (
		postgresImage          = "postgres:17-alpine@sha256:f9cf3e1f0066f0eb0c48707d864f0ac5941e9ff454a072077d97a4d278d07d73"
		redisImage             = "redis:7-alpine@sha256:7f7d30b7fdd09467bac58198a1f166dd69789d348d4fe859cccb20e0cf287fbe"
		nginxImage             = "nginx:1.30-alpine@sha256:8eb04fe3b385090e9aa02961f77398ad09b6bf33ab07f6669a5bb7acce22c0d3"
		nginxUnprivilegedImage = "nginxinc/nginx-unprivileged:1.30-alpine@sha256:f5116ed4606f3eb015dd4a8e28cb9b351476dc0ad4bc6e9f93b19cec130cdf03"
	)

	type execCheck struct {
		cmd    []string
		stdout string
	}

	// The entrypoint's temporary server used during initialization only
	// listens on the unix socket.
	postgresReady := []string{"pg_isready", "-U", "postgres", "-h", "127.0.0.1"}
	postgresChecks := []execCheck{
		{cmd: []string{"stat", "-c", "%U:%G", "/var/lib/postgresql/data"}, stdout: "postgres:postgres"},
		{cmd: []string{"psql", "-U", "postgres", "-tAc", "CREATE TABLE t AS SELECT 42 AS x"}, stdout: "SELECT 1"},
		{cmd: []string{"psql", "-U", "postgres", "-tAc", "SELECT x FROM t"}, stdout: "42"},
	}
	redisReady := []string{"redis-cli", "ping"}
	redisChecks := []execCheck{
		{cmd: []string{"stat", "-c", "%U:%G", "/data"}, stdout: "redis:redis"},
		{cmd: []string{"redis-cli", "set", "greeting", "hello"}, stdout: "OK"},
		{cmd: []string{"redis-cli", "save"}, stdout: "OK"},
	}

	// A site private to its owner, as written with a umask of 077.
	privateSite := func(t *testing.T, src string) {
		assert.NilError(t, os.Chmod(src, 0o700))
		index := filepath.Join(src, "index.html")
		assert.NilError(t, os.WriteFile(index, []byte("hello from the host\n"), 0o600))
		assert.NilError(t, os.Chown(index, hostOwner, hostOwner))
	}

	tests := []struct {
		name   string
		config *containertypes.Config
		// setup prepares the mount source before the container starts.
		setup func(t *testing.T, src string)
		// mount gets the per-case source directory as its Source.
		mount mounttypes.Mount
		// ready is retried until it succeeds before running checks.
		ready  []string
		checks []execCheck
		// created lists paths, relative to the mount source, that the
		// container must have written through the mount.
		created []string
	}{
		{
			// docker run -d -u 1000:1000 --mount type=bind,src=$SRC,dst=/app,bind-idmap busybox top
			name:   "non-root user",
			config: &containertypes.Config{Image: "busybox", User: "1000:1000", Cmd: []string{"top"}},
			mount: mounttypes.Mount{
				Type:        mounttypes.TypeBind,
				Target:      "/app",
				BindOptions: &mounttypes.BindOptions{IDMapping: &mounttypes.IDMapping{Source: mounttypes.IDMappingSourceMatchUser}},
			},
			checks: []execCheck{
				{cmd: []string{"stat", "-c", "%u:%g", "/app"}, stdout: "1000:1000"},
				{cmd: []string{"sh", "-c", "echo hi > /app/new.txt && stat -c %u:%g /app/new.txt"}, stdout: "1000:1000"},
			},
			created: []string{"new.txt"},
		},
		{
			// docker run -d --mount type=bind,src=$SRC,dst=/work,bind-idmap busybox top
			name:   "root user",
			config: &containertypes.Config{Image: "busybox", Cmd: []string{"top"}},
			mount: mounttypes.Mount{
				Type:        mounttypes.TypeBind,
				Target:      "/work",
				BindOptions: &mounttypes.BindOptions{IDMapping: &mounttypes.IDMapping{Source: mounttypes.IDMappingSourceMatchUser}},
			},
			checks: []execCheck{
				{cmd: []string{"stat", "-c", "%u:%g", "/work"}, stdout: "0:0"},
				{cmd: []string{"sh", "-c", "mkdir /work/dist && echo bin > /work/dist/app && stat -c %u:%g /work/dist/app"}, stdout: "0:0"},
			},
			created: []string{"dist/app"},
		},
		{
			// docker run -d -e POSTGRES_HOST_AUTH_METHOD=trust \
			//   --mount type=bind,src=$SRC,dst=/var/lib/postgresql/data,bind-idmap \
			//   postgres:17-alpine
			//
			// Like most official database images, postgres starts as root and
			// switches to its service user, and creates its data directory
			// owned by that user. The plain option is expected to work without
			// the user knowing that.
			name:   "postgres default user",
			config: &containertypes.Config{Image: postgresImage, Env: []string{"POSTGRES_HOST_AUTH_METHOD=trust"}},
			mount: mounttypes.Mount{
				Type:        mounttypes.TypeBind,
				Target:      "/var/lib/postgresql/data",
				BindOptions: &mounttypes.BindOptions{IDMapping: &mounttypes.IDMapping{Source: mounttypes.IDMappingSourceMatchUser}},
			},
			ready:   postgresReady,
			checks:  postgresChecks,
			created: []string{"PG_VERSION"},
		},
		{
			// docker run -d -e POSTGRES_HOST_AUTH_METHOD=trust \
			//   --mount type=bind,src=$SRC,dst=/var/lib/postgresql/data,bind-idmap=match-user:postgres \
			//   postgres:17-alpine
			name:   "postgres explicit user",
			config: &containertypes.Config{Image: postgresImage, Env: []string{"POSTGRES_HOST_AUTH_METHOD=trust"}},
			mount: mounttypes.Mount{
				Type:        mounttypes.TypeBind,
				Target:      "/var/lib/postgresql/data",
				BindOptions: &mounttypes.BindOptions{IDMapping: &mounttypes.IDMapping{Source: mounttypes.IDMappingSourceMatchUser, User: "postgres"}},
			},
			ready:   postgresReady,
			checks:  postgresChecks,
			created: []string{"PG_VERSION"},
		},
		{
			// docker run -d -u 1000:1000 -e POSTGRES_HOST_AUTH_METHOD=trust \
			//   --mount type=bind,src=$SRC,dst=/var/lib/postgresql/data,bind-idmap \
			//   postgres:17-alpine
			//
			// The image also runs as an arbitrary user, which then has to own
			// the data directory. The plain option must target the running user
			// rather than the owner of the data directory in the image.
			name: "postgres arbitrary user",
			config: &containertypes.Config{
				Image: postgresImage,
				User:  "1000:1000",
				Env:   []string{"POSTGRES_HOST_AUTH_METHOD=trust"},
			},
			mount: mounttypes.Mount{
				Type:        mounttypes.TypeBind,
				Target:      "/var/lib/postgresql/data",
				BindOptions: &mounttypes.BindOptions{IDMapping: &mounttypes.IDMapping{Source: mounttypes.IDMappingSourceMatchUser}},
			},
			ready: postgresReady,
			checks: []execCheck{
				{cmd: []string{"stat", "-c", "%u:%g", "/var/lib/postgresql/data"}, stdout: "1000:1000"},
				{cmd: []string{"psql", "-U", "postgres", "-tAc", "CREATE TABLE t AS SELECT 42 AS x"}, stdout: "SELECT 1"},
				{cmd: []string{"psql", "-U", "postgres", "-tAc", "SELECT x FROM t"}, stdout: "42"},
			},
			created: []string{"PG_VERSION"},
		},
		{
			// docker run -d --mount type=bind,src=$SRC,dst=/data,bind-idmap redis:7-alpine
			//
			// Like postgres, redis starts as root and switches to its service
			// user.
			name:   "redis default user",
			config: &containertypes.Config{Image: redisImage},
			mount: mounttypes.Mount{
				Type:        mounttypes.TypeBind,
				Target:      "/data",
				BindOptions: &mounttypes.BindOptions{IDMapping: &mounttypes.IDMapping{Source: mounttypes.IDMappingSourceMatchUser}},
			},
			ready:   redisReady,
			checks:  redisChecks,
			created: []string{"dump.rdb"},
		},
		{
			// docker run -d --mount type=bind,src=$SRC,dst=/data,bind-idmap=match-user:redis redis:7-alpine
			name:   "redis explicit user",
			config: &containertypes.Config{Image: redisImage},
			mount: mounttypes.Mount{
				Type:        mounttypes.TypeBind,
				Target:      "/data",
				BindOptions: &mounttypes.BindOptions{IDMapping: &mounttypes.IDMapping{Source: mounttypes.IDMappingSourceMatchUser, User: "redis"}},
			},
			ready:   redisReady,
			checks:  redisChecks,
			created: []string{"dump.rdb"},
		},
		{
			// docker run -d --mount type=bind,src=$SRC,dst=/usr/share/nginx/html,readonly,bind-idmap=match-user:nginx nginx:1.30-alpine
			//
			// The plain option cannot serve this site, so the user has to be
			// named. nginx starts as root and serves files from worker
			// processes that switch to the user named in nginx.conf, which the
			// daemon cannot see. Unlike a database's data directory, the
			// content directory comes from the nginx package owned by root.
			// The plain option would therefore map the site's owner to root,
			// and the workers would get 403 Forbidden.
			name:   "nginx explicit user",
			config: &containertypes.Config{Image: nginxImage},
			setup:  privateSite,
			mount: mounttypes.Mount{
				Type:        mounttypes.TypeBind,
				Target:      "/usr/share/nginx/html",
				ReadOnly:    true,
				BindOptions: &mounttypes.BindOptions{IDMapping: &mounttypes.IDMapping{Source: mounttypes.IDMappingSourceMatchUser, User: "nginx"}},
			},
			// The master process writes its pid file after binding the listen
			// sockets.
			ready: []string{"test", "-s", "/run/nginx.pid"},
			checks: []execCheck{
				{cmd: []string{"wget", "-qO-", "http://127.0.0.1/"}, stdout: "hello from the host"},
			},
		},
		{
			// docker run -d --mount type=bind,src=$SRC,dst=/usr/share/nginx/html,readonly,bind-idmap nginxinc/nginx-unprivileged:1.30-alpine
			//
			// This image runs nginx as a non-root user from the start, so the
			// plain option serves the site.
			name:   "nginx-unprivileged default user",
			config: &containertypes.Config{Image: nginxUnprivilegedImage},
			setup:  privateSite,
			mount: mounttypes.Mount{
				Type:        mounttypes.TypeBind,
				Target:      "/usr/share/nginx/html",
				ReadOnly:    true,
				BindOptions: &mounttypes.BindOptions{IDMapping: &mounttypes.IDMapping{Source: mounttypes.IDMappingSourceMatchUser}},
			},
			ready: []string{"test", "-s", "/tmp/nginx.pid"},
			checks: []execCheck{
				{cmd: []string{"wget", "-qO-", "http://127.0.0.1:8080/"}, stdout: "hello from the host"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := apiClient.ImageInspect(ctx, tc.config.Image); cerrdefs.IsNotFound(err) {
				rdr, err := apiClient.ImagePull(ctx, tc.config.Image, client.ImagePullOptions{})
				assert.NilError(t, err)
				assert.NilError(t, rdr.Wait(ctx))
			}

			// The filesystem backing the temp dir may not support id-mapped
			// mounts (overlayfs does not), so give each case its own tmpfs.
			src := t.TempDir()
			assert.NilError(t, mount.Mount("tmpfs", src, "tmpfs", fmt.Sprintf("uid=%d,gid=%d,mode=0755", hostOwner, hostOwner)))
			t.Cleanup(func() { assert.Check(t, mount.Unmount(src)) })
			if tc.setup != nil {
				tc.setup(t, src)
			}

			m := tc.mount
			m.Source = src
			ctr, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
				Config: tc.config,
				HostConfig: &containertypes.HostConfig{
					Mounts: []mounttypes.Mount{m},
					// Test directories are not labeled for SELinux.
					SecurityOpt: []string{"label=disable"},
				},
			})
			assert.NilError(t, err)

			// Entrypoint failures only show up in the container's output.
			t.Cleanup(func() {
				if !t.Failed() {
					return
				}
				logs, err := apiClient.ContainerLogs(ctx, ctr.ID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
				if err != nil {
					t.Logf("failed to get container logs: %v", err)
					return
				}
				defer logs.Close()
				var out bytes.Buffer
				_, _ = stdcopy.StdCopy(&out, &out, logs)
				t.Logf("container logs:\n%s", out.String())
			})

			_, err = apiClient.ContainerStart(ctx, ctr.ID, client.ContainerStartOptions{})
			assert.NilError(t, err)

			if tc.ready != nil {
				poll.WaitOn(t, func(poll.LogT) poll.Result {
					res, err := container.Exec(ctx, apiClient, ctr.ID, tc.ready)
					if err != nil {
						return poll.Error(err)
					}
					if res.ExitCode != 0 {
						return poll.Continue("%q: %s", tc.ready, res.Combined())
					}
					return poll.Success()
				}, poll.WithTimeout(time.Minute))
			}

			for _, check := range tc.checks {
				res := container.ExecT(ctx, t, apiClient, ctr.ID, check.cmd)
				res.AssertSuccess(t)
				assert.Check(t, is.Equal(strings.TrimSpace(res.Stdout()), check.stdout), "%q", check.cmd)
			}

			// Stop all writes to the source before inspecting it.
			_, err = apiClient.ContainerRemove(ctx, ctr.ID, client.ContainerRemoveOptions{Force: true})
			assert.NilError(t, err)

			for _, p := range tc.created {
				_, err := os.Stat(filepath.Join(src, p))
				assert.Check(t, err)
			}

			var notOwned []string
			err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				st := info.Sys().(*syscall.Stat_t)
				if st.Uid != hostOwner || st.Gid != hostOwner {
					notOwned = append(notOwned, fmt.Sprintf("%s (%d:%d)", path, st.Uid, st.Gid))
				}
				return nil
			})
			assert.NilError(t, err)
			assert.Check(t, is.Len(notOwned, 0), "not owned by %d:%d on the host", hostOwner, hostOwner)
		})
	}
}
