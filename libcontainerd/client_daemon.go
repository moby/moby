// +build !windows

package libcontainerd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/containerd/containerd"
	eventsapi "github.com/containerd/containerd/api/services/events/v1"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/linux/runcopts"
	"github.com/containerd/typeurl"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// InitProcessName is the name given to the first process of a
// container
const InitProcessName = "init"

type container struct {
	sync.Mutex

	bundleDir string
	ctr       containerd.Container
	task      containerd.Task
	execs     map[string]containerd.Process
	oomKilled bool
}

type client struct {
	sync.RWMutex // protects containers map

	remote   *containerd.Client
	stateDir string
	logger   *logrus.Entry

	namespace  string
	backend    Backend
	eventQ     queue
	containers map[string]*container
}

func (c *client) Version(ctx context.Context) (containerd.Version, error) {
	return c.remote.Version(ctx)
}

func (c *client) Restore(ctx context.Context, id string, attachStdio StdioCallback) (alive bool, pid int, err error) {
	c.Lock()
	defer c.Unlock()

	var cio containerd.IO
	defer func() {
		err = wrapError(err)
	}()

	ctr, err := c.remote.LoadContainer(ctx, id)
	if err != nil {
		return false, -1, errors.WithStack(err)
	}

	defer func() {
		if err != nil && cio != nil {
			cio.Cancel()
			cio.Close()
		}
	}()

	t, err := ctr.Task(ctx, func(fifos *containerd.FIFOSet) (containerd.IO, error) {
		io, err := newIOPipe(fifos)
		if err != nil {
			return nil, err
		}

		cio, err = attachStdio(io)
		return cio, err
	})
	if err != nil && !strings.Contains(err.Error(), "no running task found") {
		return false, -1, err
	}

	if t != nil {
		s, err := t.Status(ctx)
		if err != nil {
			return false, -1, err
		}

		alive = s.Status != containerd.Stopped
		pid = int(t.Pid())
	}
	c.containers[id] = &container{
		bundleDir: filepath.Join(c.stateDir, id),
		ctr:       ctr,
		task:      t,
		// TODO(mlaventure): load execs
	}

	c.logger.WithFields(logrus.Fields{
		"container": id,
		"alive":     alive,
		"pid":       pid,
	}).Debug("restored container")

	return alive, pid, nil
}

func (c *client) Create(ctx context.Context, id string, ociSpec *specs.Spec, runtimeOptions interface{}) error {
	if ctr := c.getContainer(id); ctr != nil {
		return errors.WithStack(newConflictError("id already in use"))
	}

	bdir, err := prepareBundleDir(filepath.Join(c.stateDir, id), ociSpec)
	if err != nil {
		return wrapSystemError(errors.Wrap(err, "prepare bundle dir failed"))
	}

	c.logger.WithField("bundle", bdir).WithField("root", ociSpec.Root.Path).Debug("bundle dir created")

	cdCtr, err := c.remote.NewContainer(ctx, id,
		containerd.WithSpec(ociSpec),
		// TODO(mlaventure): when containerd support lcow, revisit runtime value
		containerd.WithRuntime(fmt.Sprintf("io.containerd.runtime.v1.%s", runtime.GOOS), runtimeOptions))
	if err != nil {
		return err
	}

	c.Lock()
	c.containers[id] = &container{
		bundleDir: bdir,
		ctr:       cdCtr,
	}
	c.Unlock()

	return nil
}

// Start create and start a task for the specified containerd id
func (c *client) Start(ctx context.Context, id, checkpointDir string, withStdin bool, attachStdio StdioCallback) (int, error) {
	ctr := c.getContainer(id)
	switch {
	case ctr == nil:
		return -1, errors.WithStack(newNotFoundError("no such container"))
	case ctr.task != nil:
		return -1, errors.WithStack(newConflictError("container already started"))
	}

	var (
		cp             *types.Descriptor
		t              containerd.Task
		cio            containerd.IO
		err            error
		stdinCloseSync = make(chan struct{})
	)

	if checkpointDir != "" {
		// write checkpoint to the content store
		tar := archive.Diff(ctx, "", checkpointDir)
		cp, err = c.writeContent(ctx, images.MediaTypeContainerd1Checkpoint, checkpointDir, tar)
		// remove the checkpoint when we're done
		defer func() {
			if cp != nil {
				err := c.remote.ContentStore().Delete(context.Background(), cp.Digest)
				if err != nil {
					c.logger.WithError(err).WithFields(logrus.Fields{
						"ref":    checkpointDir,
						"digest": cp.Digest,
					}).Warnf("failed to delete temporary checkpoint entry")
				}
			}
		}()
		if err := tar.Close(); err != nil {
			return -1, errors.Wrap(err, "failed to close checkpoint tar stream")
		}
		if err != nil {
			return -1, errors.Wrapf(err, "failed to upload checkpoint to containerd")
		}
	}

	spec, err := ctr.ctr.Spec(ctx)
	if err != nil {
		return -1, errors.Wrap(err, "failed to retrieve spec")
	}
	uid, gid := getSpecUser(spec)
	t, err = ctr.ctr.NewTask(ctx,
		func(id string) (containerd.IO, error) {
			fifos := newFIFOSet(ctr.bundleDir, id, InitProcessName, withStdin, spec.Process.Terminal)
			cio, err = c.createIO(fifos, id, InitProcessName, stdinCloseSync, attachStdio)
			return cio, err
		},
		func(_ context.Context, _ *containerd.Client, info *containerd.TaskInfo) error {
			info.Checkpoint = cp
			info.Options = &runcopts.CreateOptions{
				IoUid: uint32(uid),
				IoGid: uint32(gid),
			}
			return nil
		})
	if err != nil {
		close(stdinCloseSync)
		if cio != nil {
			cio.Cancel()
			cio.Close()
		}
		return -1, err
	}

	c.Lock()
	c.containers[id].task = t
	c.Unlock()

	// Signal c.createIO that it can call CloseIO
	close(stdinCloseSync)

	if err := t.Start(ctx); err != nil {
		if _, err := t.Delete(ctx); err != nil {
			c.logger.WithError(err).WithField("container", id).
				Error("failed to delete task after fail start")
		}
		c.Lock()
		c.containers[id].task = nil
		c.Unlock()
		return -1, err
	}

	return int(t.Pid()), nil
}

func (c *client) Exec(ctx context.Context, containerID, processID string, spec *specs.Process, withStdin bool, attachStdio StdioCallback) (int, error) {
	ctr := c.getContainer(containerID)
	switch {
	case ctr == nil:
		return -1, errors.WithStack(newNotFoundError("no such container"))
	case ctr.task == nil:
		return -1, errors.WithStack(newInvalidParameterError("container is not running"))
	case ctr.execs != nil && ctr.execs[processID] != nil:
		return -1, errors.WithStack(newConflictError("id already in use"))
	}

	var (
		p              containerd.Process
		cio            containerd.IO
		err            error
		stdinCloseSync = make(chan struct{})
	)

	fifos := newFIFOSet(ctr.bundleDir, containerID, processID, withStdin, spec.Terminal)

	defer func() {
		if err != nil {
			if cio != nil {
				cio.Cancel()
				cio.Close()
			}
			rmFIFOSet(fifos)
		}
	}()

	p, err = ctr.task.Exec(ctx, processID, spec, func(id string) (containerd.IO, error) {
		cio, err = c.createIO(fifos, containerID, processID, stdinCloseSync, attachStdio)
		return cio, err
	})
	if err != nil {
		close(stdinCloseSync)
		if cio != nil {
			cio.Cancel()
			cio.Close()
		}
		return -1, err
	}

	ctr.Lock()
	if ctr.execs == nil {
		ctr.execs = make(map[string]containerd.Process)
	}
	ctr.execs[processID] = p
	ctr.Unlock()

	// Signal c.createIO that it can call CloseIO
	close(stdinCloseSync)

	if err = p.Start(ctx); err != nil {
		p.Delete(context.Background())
		ctr.Lock()
		delete(ctr.execs, processID)
		ctr.Unlock()
		return -1, err
	}

	return int(p.Pid()), nil
}

func (c *client) SignalProcess(ctx context.Context, containerID, processID string, signal int) error {
	p, err := c.getProcess(containerID, processID)
	if err != nil {
		return err
	}
	return p.Kill(ctx, syscall.Signal(signal))
}

func (c *client) ResizeTerminal(ctx context.Context, containerID, processID string, width, height int) error {
	p, err := c.getProcess(containerID, processID)
	if err != nil {
		return err
	}

	return p.Resize(ctx, uint32(width), uint32(height))
}

func (c *client) CloseStdin(ctx context.Context, containerID, processID string) error {
	p, err := c.getProcess(containerID, processID)
	if err != nil {
		return err
	}

	return p.CloseIO(ctx, containerd.WithStdinCloser)
}

func (c *client) Pause(ctx context.Context, containerID string) error {
	p, err := c.getProcess(containerID, InitProcessName)
	if err != nil {
		return err
	}

	return p.(containerd.Task).Pause(ctx)
}

func (c *client) Resume(ctx context.Context, containerID string) error {
	p, err := c.getProcess(containerID, InitProcessName)
	if err != nil {
		return err
	}

	return p.(containerd.Task).Resume(ctx)
}

func (c *client) Stats(ctx context.Context, containerID string) (*Stats, error) {
	p, err := c.getProcess(containerID, InitProcessName)
	if err != nil {
		return nil, err
	}

	m, err := p.(containerd.Task).Metrics(ctx)
	if err != nil {
		return nil, err
	}

	v, err := typeurl.UnmarshalAny(m.Data)
	if err != nil {
		return nil, err
	}
	return interfaceToStats(m.Timestamp, v), nil
}

func (c *client) ListPids(ctx context.Context, containerID string) ([]uint32, error) {
	p, err := c.getProcess(containerID, InitProcessName)
	if err != nil {
		return nil, err
	}

	pis, err := p.(containerd.Task).Pids(ctx)
	if err != nil {
		return nil, err
	}

	var pids []uint32
	for _, i := range pis {
		pids = append(pids, i.Pid)
	}

	return pids, nil
}

func (c *client) Summary(ctx context.Context, containerID string) ([]Summary, error) {
	p, err := c.getProcess(containerID, InitProcessName)
	if err != nil {
		return nil, err
	}

	pis, err := p.(containerd.Task).Pids(ctx)
	if err != nil {
		return nil, err
	}

	var infos []Summary
	for _, pi := range pis {
		i, err := typeurl.UnmarshalAny(pi.Info)
		if err != nil {
			return nil, errors.Wrap(err, "unable to decode process details")
		}
		s, err := summaryFromInterface(i)
		if err != nil {
			return nil, err
		}
		infos = append(infos, *s)
	}

	return infos, nil
}

func (c *client) DeleteTask(ctx context.Context, containerID string) (uint32, time.Time, error) {
	p, err := c.getProcess(containerID, InitProcessName)
	if err != nil {
		return 255, time.Now(), nil
	}

	status, err := p.(containerd.Task).Delete(ctx)
	if err != nil {
		return 255, time.Now(), nil
	}

	c.Lock()
	if ctr, ok := c.containers[containerID]; ok {
		ctr.task = nil
	}
	c.Unlock()

	return status.ExitCode(), status.ExitTime(), nil
}

func (c *client) Delete(ctx context.Context, containerID string) error {
	ctr := c.getContainer(containerID)
	if ctr == nil {
		return errors.WithStack(newNotFoundError("no such container"))
	}

	if err := ctr.ctr.Delete(ctx); err != nil {
		return err
	}

	if os.Getenv("LIBCONTAINERD_NOCLEAN") != "1" {
		if err := os.RemoveAll(ctr.bundleDir); err != nil {
			c.logger.WithError(err).WithFields(logrus.Fields{
				"container": containerID,
				"bundle":    ctr.bundleDir,
			}).Error("failed to remove state dir")
		}
	}

	c.removeContainer(containerID)

	return nil
}

func (c *client) Status(ctx context.Context, containerID string) (Status, error) {
	ctr := c.getContainer(containerID)
	if ctr == nil {
		return StatusUnknown, errors.WithStack(newNotFoundError("no such container"))
	}

	s, err := ctr.task.Status(ctx)
	if err != nil {
		return StatusUnknown, err
	}

	return Status(s.Status), nil
}

func (c *client) CreateCheckpoint(ctx context.Context, containerID, checkpointDir string, exit bool) error {
	p, err := c.getProcess(containerID, InitProcessName)
	if err != nil {
		return err
	}

	img, err := p.(containerd.Task).Checkpoint(ctx)
	if err != nil {
		return err
	}
	// Whatever happens, delete the checkpoint from containerd
	defer func() {
		err := c.remote.ImageService().Delete(context.Background(), img.Name())
		if err != nil {
			c.logger.WithError(err).WithField("digest", img.Target().Digest).
				Warnf("failed to delete checkpoint image")
		}
	}()

	b, err := content.ReadBlob(ctx, c.remote.ContentStore(), img.Target().Digest)
	if err != nil {
		return wrapSystemError(errors.Wrapf(err, "failed to retrieve checkpoint data"))
	}
	var index v1.Index
	if err := json.Unmarshal(b, &index); err != nil {
		return wrapSystemError(errors.Wrapf(err, "failed to decode checkpoint data"))
	}

	var cpDesc *v1.Descriptor
	for _, m := range index.Manifests {
		if m.MediaType == images.MediaTypeContainerd1Checkpoint {
			cpDesc = &m
			break
		}
	}
	if cpDesc == nil {
		return wrapSystemError(errors.Wrapf(err, "invalid checkpoint"))
	}

	rat, err := c.remote.ContentStore().ReaderAt(ctx, cpDesc.Digest)
	if err != nil {
		return wrapSystemError(errors.Wrapf(err, "failed to get checkpoint reader"))
	}
	defer rat.Close()
	_, err = archive.Apply(ctx, checkpointDir, content.NewReader(rat))
	if err != nil {
		return wrapSystemError(errors.Wrapf(err, "failed to read checkpoint reader"))
	}

	return err
}

func (c *client) getContainer(id string) *container {
	c.RLock()
	ctr := c.containers[id]
	c.RUnlock()

	return ctr
}

func (c *client) removeContainer(id string) {
	c.Lock()
	delete(c.containers, id)
	c.Unlock()
}

func (c *client) getProcess(containerID, processID string) (containerd.Process, error) {
	ctr := c.getContainer(containerID)
	switch {
	case ctr == nil:
		return nil, errors.WithStack(newNotFoundError("no such container"))
	case ctr.task == nil:
		return nil, errors.WithStack(newNotFoundError("container is not running"))
	case processID == InitProcessName:
		return ctr.task, nil
	default:
		ctr.Lock()
		defer ctr.Unlock()
		if ctr.execs == nil {
			return nil, errors.WithStack(newNotFoundError("no execs"))
		}
	}

	p := ctr.execs[processID]
	if p == nil {
		return nil, errors.WithStack(newNotFoundError("no such exec"))
	}

	return p, nil
}

// createIO creates the io to be used by a process
// This needs to get a pointer to interface as upon closure the process may not have yet been registered
func (c *client) createIO(fifos *containerd.FIFOSet, containerID, processID string, stdinCloseSync chan struct{}, attachStdio StdioCallback) (containerd.IO, error) {
	io, err := newIOPipe(fifos)
	if err != nil {
		return nil, err
	}

	if io.Stdin != nil {
		var (
			err       error
			stdinOnce sync.Once
		)
		pipe := io.Stdin
		io.Stdin = ioutils.NewWriteCloserWrapper(pipe, func() error {
			stdinOnce.Do(func() {
				err = pipe.Close()
				// Do the rest in a new routine to avoid a deadlock if the
				// Exec/Start call failed.
				go func() {
					<-stdinCloseSync
					p, err := c.getProcess(containerID, processID)
					if err == nil {
						err = p.CloseIO(context.Background(), containerd.WithStdinCloser)
						if err != nil && strings.Contains(err.Error(), "transport is closing") {
							err = nil
						}
					}
				}()
			})
			return err
		})
	}

	cio, err := attachStdio(io)
	if err != nil {
		io.Cancel()
		io.Close()
	}
	return cio, err
}

func (c *client) processEvent(ctr *container, et EventType, ei EventInfo) {
	c.eventQ.append(ei.ContainerID, func() {
		err := c.backend.ProcessEvent(ei.ContainerID, et, ei)
		if err != nil {
			c.logger.WithError(err).WithFields(logrus.Fields{
				"container":  ei.ContainerID,
				"event":      et,
				"event-info": ei,
			}).Error("failed to process event")
		}

		if et == EventExit && ei.ProcessID != ei.ContainerID {
			var p containerd.Process
			ctr.Lock()
			if ctr.execs != nil {
				p = ctr.execs[ei.ProcessID]
			}
			ctr.Unlock()
			if p == nil {
				c.logger.WithError(errors.New("no such process")).
					WithFields(logrus.Fields{
						"container": ei.ContainerID,
						"process":   ei.ProcessID,
					}).Error("exit event")
				return
			}
			_, err = p.Delete(context.Background())
			if err != nil {
				c.logger.WithError(err).WithFields(logrus.Fields{
					"container": ei.ContainerID,
					"process":   ei.ProcessID,
				}).Warn("failed to delete process")
			}
			c.Lock()
			delete(ctr.execs, ei.ProcessID)
			c.Unlock()
			ctr := c.getContainer(ei.ContainerID)
			if ctr == nil {
				c.logger.WithFields(logrus.Fields{
					"container": ei.ContainerID,
				}).Error("failed to find container")
			} else {
				rmFIFOSet(newFIFOSet(ctr.bundleDir, ei.ContainerID, ei.ProcessID, true, false))
			}
		}
	})
}

func (c *client) processEventStream(ctx context.Context) {
	var (
		err         error
		eventStream eventsapi.Events_SubscribeClient
		ev          *eventsapi.Envelope
		et          EventType
		ei          EventInfo
		ctr         *container
	)
	defer func() {
		if err != nil {
			select {
			case <-ctx.Done():
				c.logger.WithError(ctx.Err()).
					Info("stopping event stream following graceful shutdown")
			default:
				go c.processEventStream(ctx)
			}
		}
	}()

	eventStream, err = c.remote.EventService().Subscribe(ctx, &eventsapi.SubscribeRequest{
		Filters: []string{"namespace==" + c.namespace + ",topic~=/tasks/.+"},
	}, grpc.FailFast(false))
	if err != nil {
		return
	}

	var oomKilled bool
	for {
		ev, err = eventStream.Recv()
		if err != nil {
			errStatus, ok := status.FromError(err)
			if !ok || errStatus.Code() != codes.Canceled {
				c.logger.WithError(err).Error("failed to get event")
			}
			return
		}

		if ev.Event == nil {
			c.logger.WithField("event", ev).Warn("invalid event")
			continue
		}

		v, err := typeurl.UnmarshalAny(ev.Event)
		if err != nil {
			c.logger.WithError(err).WithField("event", ev).Warn("failed to unmarshal event")
			continue
		}

		c.logger.WithField("topic", ev.Topic).Debug("event")

		switch t := v.(type) {
		case *eventsapi.TaskCreate:
			et = EventCreate
			ei = EventInfo{
				ContainerID: t.ContainerID,
				ProcessID:   t.ContainerID,
				Pid:         t.Pid,
			}
		case *eventsapi.TaskStart:
			et = EventStart
			ei = EventInfo{
				ContainerID: t.ContainerID,
				ProcessID:   t.ContainerID,
				Pid:         t.Pid,
			}
		case *eventsapi.TaskExit:
			et = EventExit
			ei = EventInfo{
				ContainerID: t.ContainerID,
				ProcessID:   t.ID,
				Pid:         t.Pid,
				ExitCode:    t.ExitStatus,
				ExitedAt:    t.ExitedAt,
			}
		case *eventsapi.TaskOOM:
			et = EventOOM
			ei = EventInfo{
				ContainerID: t.ContainerID,
				OOMKilled:   true,
			}
			oomKilled = true
		case *eventsapi.TaskExecAdded:
			et = EventExecAdded
			ei = EventInfo{
				ContainerID: t.ContainerID,
				ProcessID:   t.ExecID,
			}
		case *eventsapi.TaskExecStarted:
			et = EventExecStarted
			ei = EventInfo{
				ContainerID: t.ContainerID,
				ProcessID:   t.ExecID,
				Pid:         t.Pid,
			}
		case *eventsapi.TaskPaused:
			et = EventPaused
			ei = EventInfo{
				ContainerID: t.ContainerID,
			}
		case *eventsapi.TaskResumed:
			et = EventResumed
			ei = EventInfo{
				ContainerID: t.ContainerID,
			}
		default:
			c.logger.WithFields(logrus.Fields{
				"topic": ev.Topic,
				"type":  reflect.TypeOf(t)},
			).Info("ignoring event")
			continue
		}

		ctr = c.getContainer(ei.ContainerID)
		if ctr == nil {
			c.logger.WithField("container", ei.ContainerID).Warn("unknown container")
			continue
		}

		if oomKilled {
			ctr.oomKilled = true
			oomKilled = false
		}
		ei.OOMKilled = ctr.oomKilled

		c.processEvent(ctr, et, ei)
	}
}

func (c *client) writeContent(ctx context.Context, mediaType, ref string, r io.Reader) (*types.Descriptor, error) {
	writer, err := c.remote.ContentStore().Writer(ctx, ref, 0, "")
	if err != nil {
		return nil, err
	}
	defer writer.Close()
	size, err := io.Copy(writer, r)
	if err != nil {
		return nil, err
	}
	labels := map[string]string{
		"containerd.io/gc.root": time.Now().UTC().Format(time.RFC3339),
	}
	if err := writer.Commit(ctx, 0, "", content.WithLabels(labels)); err != nil {
		return nil, err
	}
	return &types.Descriptor{
		MediaType: mediaType,
		Digest:    writer.Digest(),
		Size_:     size,
	}, nil
}

func wrapError(err error) error {
	if err != nil {
		msg := err.Error()
		for _, s := range []string{"container does not exist", "not found", "no such container"} {
			if strings.Contains(msg, s) {
				return wrapNotFoundError(err)
			}
		}
	}
	return err
}
