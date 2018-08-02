package runcexecutor

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/containerd/containerd/contrib/seccomp"
	"github.com/containerd/containerd/mount"
	containerdoci "github.com/containerd/containerd/oci"
	"github.com/containerd/continuity/fs"
	runc "github.com/containerd/go-runc"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/identity"
	rootlessspecconv "github.com/moby/buildkit/util/rootless/specconv"
	"github.com/moby/buildkit/util/system"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Opt struct {
	// root directory
	Root              string
	CommandCandidates []string
	// without root privileges (has nothing to do with Opt.Root directory)
	Rootless bool
}

var defaultCommandCandidates = []string{"buildkit-runc", "runc"}

type runcExecutor struct {
	runc     *runc.Runc
	root     string
	cmd      string
	rootless bool
}

func New(opt Opt) (executor.Executor, error) {
	cmds := opt.CommandCandidates
	if cmds == nil {
		cmds = defaultCommandCandidates
	}

	var cmd string
	var found bool
	for _, cmd = range cmds {
		if _, err := exec.LookPath(cmd); err == nil {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.Errorf("failed to find %s binary", cmd)
	}

	root := opt.Root

	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, errors.Wrapf(err, "failed to create %s", root)
	}

	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}

	runtime := &runc.Runc{
		Command:      cmd,
		Log:          filepath.Join(root, "runc-log.json"),
		LogFormat:    runc.JSON,
		PdeathSignal: syscall.SIGKILL,
		Setpgid:      true,
		// we don't execute runc with --rootless=(true|false) explicitly,
		// so as to support non-runc runtimes
	}

	w := &runcExecutor{
		runc:     runtime,
		root:     root,
		rootless: opt.Rootless,
	}
	return w, nil
}

func (w *runcExecutor) Exec(ctx context.Context, meta executor.Meta, root cache.Mountable, mounts []executor.Mount, stdin io.ReadCloser, stdout, stderr io.WriteCloser) error {

	resolvConf, err := oci.GetResolvConf(ctx, w.root)
	if err != nil {
		return err
	}

	hostsFile, err := oci.GetHostsFile(ctx, w.root)
	if err != nil {
		return err
	}

	mountable, err := root.Mount(ctx, false)
	if err != nil {
		return err
	}

	rootMount, err := mountable.Mount()
	if err != nil {
		return err
	}
	defer mountable.Release()

	id := identity.NewID()
	bundle := filepath.Join(w.root, id)

	if err := os.Mkdir(bundle, 0700); err != nil {
		return err
	}
	defer os.RemoveAll(bundle)
	rootFSPath := filepath.Join(bundle, "rootfs")
	if err := os.Mkdir(rootFSPath, 0700); err != nil {
		return err
	}
	if err := mount.All(rootMount, rootFSPath); err != nil {
		return err
	}
	defer mount.Unmount(rootFSPath, 0)

	uid, gid, sgids, err := oci.GetUser(ctx, rootFSPath, meta.User)
	if err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(bundle, "config.json"))
	if err != nil {
		return err
	}
	defer f.Close()
	opts := []containerdoci.SpecOpts{oci.WithUIDGID(uid, gid, sgids)}
	if system.SeccompSupported() {
		opts = append(opts, seccomp.WithDefaultProfile())
	}
	if meta.ReadonlyRootFS {
		opts = append(opts, containerdoci.WithRootFSReadonly())
	}
	spec, cleanup, err := oci.GenerateSpec(ctx, meta, mounts, id, resolvConf, hostsFile, opts...)
	if err != nil {
		return err
	}
	defer cleanup()

	spec.Root.Path = rootFSPath
	if _, ok := root.(cache.ImmutableRef); ok { // TODO: pass in with mount, not ref type
		spec.Root.Readonly = true
	}

	newp, err := fs.RootPath(rootFSPath, meta.Cwd)
	if err != nil {
		return errors.Wrapf(err, "working dir %s points to invalid target", newp)
	}
	if err := os.MkdirAll(newp, 0755); err != nil {
		return errors.Wrapf(err, "failed to create working directory %s", newp)
	}

	if err := setOOMScoreAdj(spec); err != nil {
		return err
	}
	if w.rootless {
		if err := rootlessspecconv.ToRootless(spec); err != nil {
			return err
		}
	}

	if err := json.NewEncoder(f).Encode(spec); err != nil {
		return err
	}

	logrus.Debugf("> running %s %v", id, meta.Args)

	status, err := w.runc.Run(ctx, id, bundle, &runc.CreateOpts{
		IO: &forwardIO{stdin: stdin, stdout: stdout, stderr: stderr},
	})
	logrus.Debugf("< completed %s %v %v", id, status, err)
	if status != 0 {
		select {
		case <-ctx.Done():
			// runc can't report context.Cancelled directly
			return errors.Wrapf(ctx.Err(), "exit code %d", status)
		default:
		}
		return errors.Errorf("exit code %d", status)
	}

	return err
}

type forwardIO struct {
	stdin          io.ReadCloser
	stdout, stderr io.WriteCloser
}

func (s *forwardIO) Close() error {
	return nil
}

func (s *forwardIO) Set(cmd *exec.Cmd) {
	cmd.Stdin = s.stdin
	cmd.Stdout = s.stdout
	cmd.Stderr = s.stderr
}

func (s *forwardIO) Stdin() io.WriteCloser {
	return nil
}

func (s *forwardIO) Stdout() io.ReadCloser {
	return nil
}

func (s *forwardIO) Stderr() io.ReadCloser {
	return nil
}

// setOOMScoreAdj comes from https://github.com/genuinetools/img/blob/2fabe60b7dc4623aa392b515e013bbc69ad510ab/executor/runc/executor.go#L182-L192
func setOOMScoreAdj(spec *specs.Spec) error {
	// Set the oom_score_adj of our children containers to that of the current process.
	b, err := ioutil.ReadFile("/proc/self/oom_score_adj")
	if err != nil {
		return errors.Wrap(err, "failed to read /proc/self/oom_score_adj")
	}
	s := strings.TrimSpace(string(b))
	oom, err := strconv.Atoi(s)
	if err != nil {
		return errors.Wrapf(err, "failed to parse %s as int", s)
	}
	spec.Process.OOMScoreAdj = &oom
	return nil
}
