package containerd // import "github.com/docker/docker/plugin/executor/containerd"

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/runtime/linux/runctypes"
	"github.com/docker/docker/pkg/idtools"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

// PluginNamespace is the name used for the plugins namespace
const PluginNamespace = "plugins.moby"

// ExitHandler represents an object that is called when the exit event is received from containerd
type ExitHandler interface {
	HandleExitEvent(id string) error
}

// New creates a new containerd plugin executor
func New(ctx context.Context, stateDir string, cli *containerd.Client, exitHandler ExitHandler) (*Executor, error) {
	return &Executor{
		stateDir:    stateDir,
		exitHandler: exitHandler,
		client:      cli,
	}, nil
}

// Executor is the containerd client implementation of a plugin executor
type Executor struct {
	mu sync.Mutex

	stateDir    string
	client      *containerd.Client
	exitHandler ExitHandler
}

// Create creates a new container
func (e *Executor) Create(id string, spec specs.Spec, stdout, stderr io.WriteCloser) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	opts := &runctypes.RuncOptions{
		RuntimeRoot: filepath.Join(e.stateDir, "runtime-root"),
	}
	if _, err := prepareBundleDir(filepath.Join(e.stateDir, id), &spec); err != nil {
		return err
	}
	ctx := context.Background()
	container, err := e.client.NewContainer(ctx, id,
		containerd.WithSpec(&spec),
		containerd.WithRuntime(fmt.Sprintf("io.containerd.runtime.v1.%s", runtime.GOOS), opts),
	)
	if err != nil {
		if !errdefs.IsAlreadyExists(errdefs.FromGRPC(err)) {
			return err
		}
		if container, err = e.client.LoadContainer(ctx, id); err != nil {
			return err
		}
	}
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStreams(nil, stdout, stderr)))
	if err != nil {
		container.Delete(ctx)
		return err
	}
	wait, err := task.Wait(ctx)
	if err != nil {
		task.Delete(ctx)
		container.Delete(ctx)
		return err
	}
	go func() {
		<-wait
		e.mu.Lock()
		stdout.Close()
		stderr.Close()
		if _, err := task.Delete(ctx); err != nil {
			logrus.WithError(err).WithField("id", id).Error("failed to handle delete task")
		}
		if err := container.Delete(ctx); err != nil {
			logrus.WithError(err).WithField("id", id).Error("failed to handle delete container")
		}
		e.mu.Unlock()
		if err := e.exitHandler.HandleExitEvent(id); err != nil {
			logrus.WithError(err).WithField("id", id).Error("failed to handle exit event")
		}
	}()
	if err := task.Start(ctx); err != nil {
		task.Delete(ctx)
		<-wait
		container.Delete(ctx)
		return err
	}
	return nil
}

// Restore restores a container
func (e *Executor) Restore(id string, stdout, stderr io.WriteCloser) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ctx := context.Background()
	container, err := e.client.LoadContainer(ctx, id)
	if err != nil {
		return false, err
	}
	task, err := container.Task(ctx, cio.NewAttach(cio.WithStreams(nil, stdout, stderr)))
	if err != nil {
		container.Delete(ctx)
		return false, err
	}

	status, err := task.Status(ctx)
	if err != nil {
		logrus.WithError(err).WithField("id", id).Error("failed to stat task")
		if _, err := task.Delete(ctx, containerd.WithProcessKill); err != nil {
			logrus.WithError(err).WithField("id", id).Error("failed to handle delete task")
		}
		return true, container.Delete(ctx)
	}
	if status.Status != containerd.Running {
		if _, err := task.Delete(ctx); err != nil {
			logrus.WithError(err).WithField("id", id).Error("failed to handle delete task")
		}
		return true, container.Delete(ctx)
	}
	wait, err := task.Wait(ctx)
	if err != nil {
		logrus.WithError(err).WithField("id", id).Error("failed to wait task")
		return true, nil
	}
	go func() {
		<-wait
		e.mu.Lock()
		stdout.Close()
		stderr.Close()
		if _, err := task.Delete(ctx); err != nil {
			logrus.WithError(err).WithField("id", id).Error("failed to handle delete task")
		}
		if err := container.Delete(ctx); err != nil {
			logrus.WithError(err).WithField("id", id).Error("failed to handle delete container")
		}
		e.mu.Unlock()
		if err := e.exitHandler.HandleExitEvent(id); err != nil {
			logrus.WithError(err).WithField("id", id).Error("failed to handle exit event")
		}

	}()
	return true, nil
}

// IsRunning returns if the container with the given id is running
func (e *Executor) IsRunning(id string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ctx := context.Background()
	container, err := e.client.LoadContainer(ctx, id)
	if err != nil {
		return false, err
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		return false, err
	}
	status, err := task.Status(ctx)
	if err != nil {
		return false, err
	}
	return status.Status == containerd.Running, nil
}

// Signal sends the specified signal to the container
func (e *Executor) Signal(id string, signal int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	ctx := context.Background()
	container, err := e.client.LoadContainer(ctx, id)
	if err != nil {
		return err
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}
	return task.Kill(ctx, syscall.Signal(signal))
}

func prepareBundleDir(bundleDir string, ociSpec *specs.Spec) (string, error) {
	uid, gid := getSpecUser(ociSpec)
	if uid == 0 && gid == 0 {
		return bundleDir, idtools.MkdirAllAndChownNew(bundleDir, 0755, idtools.Identity{UID: 0, GID: 0})
	}

	p := string(filepath.Separator)
	components := strings.Split(bundleDir, string(filepath.Separator))
	for _, d := range components[1:] {
		p = filepath.Join(p, d)
		fi, err := os.Stat(p)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if os.IsNotExist(err) || fi.Mode()&1 == 0 {
			p = fmt.Sprintf("%s.%d.%d", p, uid, gid)
			if err := idtools.MkdirAndChown(p, 0700, idtools.Identity{UID: uid, GID: gid}); err != nil && !os.IsExist(err) {
				return "", err
			}
		}
	}
	return p, nil
}

func getSpecUser(ociSpec *specs.Spec) (int, int) {
	var (
		uid int
		gid int
	)

	for _, ns := range ociSpec.Linux.Namespaces {
		if ns.Type == specs.UserNamespace {
			uid = hostIDFromMap(0, ociSpec.Linux.UIDMappings)
			gid = hostIDFromMap(0, ociSpec.Linux.GIDMappings)
			break
		}
	}

	return uid, gid
}

func hostIDFromMap(id uint32, mp []specs.LinuxIDMapping) int {
	for _, m := range mp {
		if id >= m.ContainerID && id <= m.ContainerID+m.Size-1 {
			return int(m.HostID + id - m.ContainerID)
		}
	}
	return 0
}
