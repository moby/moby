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
	"sync"
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
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/network"
	rootlessspecconv "github.com/moby/buildkit/util/rootless/specconv"
	"github.com/moby/buildkit/util/system"
	runcsystem "github.com/opencontainers/runc/libcontainer/system"
	specs "github.com/opencontainers/runtime-spec/specs-go"
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
	runc            *runc.Runc
	root            string
	cmd             string
	rootless        bool
	networkProvider network.Provider
}

func New(opt Opt, networkProvider network.Provider) (executor.Executor, error) {
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

	if err := setSubReaper(); err != nil {
		return nil, err
	}

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
		runc:            runtime,
		root:            root,
		rootless:        opt.Rootless,
		networkProvider: networkProvider,
	}
	return w, nil
}

func (w *runcExecutor) Exec(ctx context.Context, meta executor.Meta, root cache.Mountable, mounts []executor.Mount, stdin io.ReadCloser, stdout, stderr io.WriteCloser) error {
	var iface network.Interface
	// FIXME: still uses host if no provider configured
	if meta.NetMode == pb.NetMode_UNSET {
		if w.networkProvider != nil {
			var err error
			iface, err = w.networkProvider.NewInterface()
			if err != nil || iface == nil {
				meta.NetMode = pb.NetMode_HOST
			}
		} else {
			meta.NetMode = pb.NetMode_HOST
		}
	}
	if meta.NetMode == pb.NetMode_HOST {
		logrus.Info("enabling HostNetworking")
	}
	defer func() {
		if iface != nil {
			w.networkProvider.Release(iface)
		}
	}()

	resolvConf, err := oci.GetResolvConf(ctx, w.root)
	if err != nil {
		return err
	}

	hostsFile, clean, err := oci.GetHostsFile(ctx, w.root, meta.ExtraHosts)
	if err != nil {
		return err
	}
	if clean != nil {
		defer clean()
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
	spec, cleanup, err := oci.GenerateSpec(ctx, meta, mounts, id, resolvConf, hostsFile, meta.NetMode == pb.NetMode_HOST, opts...)
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

	forwardIO, err := newForwardIO(stdin, stdout, stderr)
	if err != nil {
		return errors.Wrap(err, "creating new forwarding IO")
	}
	defer forwardIO.Close()

	pidFilePath := filepath.Join(w.root, "runc_pid_"+identity.NewID())
	defer os.RemoveAll(pidFilePath)

	logrus.Debugf("> creating %s %v", id, meta.Args)
	err = w.runc.Create(ctx, id, bundle, &runc.CreateOpts{
		PidFile: pidFilePath,
		IO:      forwardIO,
	})
	if err != nil {
		return err
	}
	forwardIO.release()

	defer func() {
		go func() {
			if err := w.runc.Delete(context.TODO(), id, &runc.DeleteOpts{}); err != nil {
				logrus.Errorf("failed to delete %s: %+v", id, err)
			}
		}()
	}()

	dt, err := ioutil.ReadFile(pidFilePath)
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(string(dt))
	if err != nil {
		return err
	}

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-done:
		case <-ctx.Done():
			syscall.Kill(-pid, syscall.SIGKILL)
		}
	}()

	if iface != nil {
		if err := iface.Set(pid); err != nil {
			return errors.Wrap(err, "could not set the network")
		}
		defer func() {
			iface.Remove(pid)
		}()
	}

	err = w.runc.Start(ctx, id)
	if err != nil {
		return err
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	status := 0
	ps, err := p.Wait()
	if err != nil {
		status = 255
	}

	if ws, ok := ps.Sys().(syscall.WaitStatus); ok {
		status = ws.ExitStatus()
	}
	if status != 0 {
		return errors.Errorf("exit code: %d", status)
	}

	return nil
}

type forwardIO struct {
	stdin, stdout, stderr *os.File
	toRelease             []io.Closer
	toClose               []io.Closer
}

func newForwardIO(stdin io.ReadCloser, stdout, stderr io.WriteCloser) (f *forwardIO, err error) {
	fio := &forwardIO{}
	defer func() {
		if err != nil {
			fio.Close()
		}
	}()
	if stdin != nil {
		fio.stdin, err = fio.readCloserToFile(stdin)
		if err != nil {
			return nil, err
		}
	}
	if stdout != nil {
		fio.stdout, err = fio.writeCloserToFile(stdout)
		if err != nil {
			return nil, err
		}
	}
	if stderr != nil {
		fio.stderr, err = fio.writeCloserToFile(stderr)
		if err != nil {
			return nil, err
		}
	}
	return fio, nil
}

func (s *forwardIO) Close() error {
	s.release()
	var err error
	for _, cl := range s.toClose {
		if err1 := cl.Close(); err == nil {
			err = err1
		}
	}
	s.toClose = nil
	return err
}

// release releases active FDs if the process doesn't need them any more
func (s *forwardIO) release() {
	for _, cl := range s.toRelease {
		cl.Close()
	}
	s.toRelease = nil
}

func (s *forwardIO) Set(cmd *exec.Cmd) {
	cmd.Stdin = s.stdin
	cmd.Stdout = s.stdout
	cmd.Stderr = s.stderr
}

func (s *forwardIO) readCloserToFile(rc io.ReadCloser) (*os.File, error) {
	if f, ok := rc.(*os.File); ok {
		return f, nil
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	s.toClose = append(s.toClose, pw)
	s.toRelease = append(s.toRelease, pr)
	go func() {
		_, err := io.Copy(pw, rc)
		if err1 := pw.Close(); err == nil {
			err = err1
		}
		_ = err
	}()
	return pr, nil
}

func (s *forwardIO) writeCloserToFile(wc io.WriteCloser) (*os.File, error) {
	if f, ok := wc.(*os.File); ok {
		return f, nil
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	s.toClose = append(s.toClose, pr)
	s.toRelease = append(s.toRelease, pw)
	go func() {
		_, err := io.Copy(wc, pr)
		if err1 := pw.Close(); err == nil {
			err = err1
		}
		_ = err
	}()
	return pw, nil
}

var subReaperOnce sync.Once
var subReaperError error

func setSubReaper() error {
	subReaperOnce.Do(func() {
		subReaperError = runcsystem.SetSubreaper(1)
	})
	return subReaperError
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
