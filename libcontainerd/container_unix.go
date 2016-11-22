// +build linux solaris

package libcontainerd

import (
	"encoding/json"
        "fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	libvirtgo "github.com/rgbkrk/libvirt-go"
	containerd "github.com/docker/containerd/api/grpc/types"
	"github.com/docker/docker/pkg/ioutils"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/tonistiigi/fifo"
	"golang.org/x/net/context"
)

// Required for isolated container.
type IcContext interface {
	CreateSeedImage(seedDirectory string) (string, error)
	CreateDeltaDiskImage(deltaDiskDirectory, diskPath string) (string, error)
	DomainXml() (string, error)
	GetDomain() (*libvirtgo.VirDomain)
	GetQemuDirectory() string
	Create()
	Launch()
	Shutdown()
	Close()
	Pause(pause bool) error
}

type container struct {
	containerCommon

        // Domain for isolated container
        isolatedContainerContext IcContext

	// Platform specific fields are below here.
	pauseMonitor
	oom         bool
	runtime     string
	runtimeArgs []string
}

type runtime struct {
	path string
	args []string
}

// WithRuntime sets the runtime to be used for the created container
func WithRuntime(path string, args []string) CreateOption {
	return runtime{path, args}
}

func (rt runtime) Apply(p interface{}) error {
	if pr, ok := p.(*container); ok {
		pr.runtime = rt.path
		pr.runtimeArgs = rt.args
	}
	return nil
}

func (ctr *container) clean() error {
	if os.Getenv("LIBCONTAINERD_NOCLEAN") == "1" {
		return nil
	}
	if _, err := os.Lstat(ctr.dir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := os.RemoveAll(ctr.dir); err != nil {
		return err
	}
	return nil
}

// cleanProcess removes the fifos used by an additional process.
// Caller needs to lock container ID before calling this method.
func (ctr *container) cleanProcess(id string) {
	if p, ok := ctr.processes[id]; ok {
		for _, i := range []int{syscall.Stdin, syscall.Stdout, syscall.Stderr} {
			if err := os.Remove(p.fifo(i)); err != nil && !os.IsNotExist(err) {
				logrus.Warnf("libcontainerd: failed to remove %v for process %v: %v", p.fifo(i), id, err)
			}
		}
	}
	delete(ctr.processes, id)
}

func (ctr *container) spec() (*specs.Spec, error) {
	var spec specs.Spec
	dt, err := ioutil.ReadFile(filepath.Join(ctr.dir, configFilename))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(dt, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func (ctr *container) startQemu() error {
	spec, err := ctr.spec()
	if err != nil {
		return nil
	}
        logrus.Debugf("The container spec is: %s", spec)

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	if ctr.isolatedContainerContext != nil {
		seedImageLocation, err := ctr.isolatedContainerContext.CreateSeedImage(ctr.isolatedContainerContext.GetQemuDirectory())
		if err != nil {
			return fmt.Errorf("Could not create seed image : %s", err)
		}

		logrus.Infof("Seed image location: %s", seedImageLocation)

		ctr.isolatedContainerContext.Launch()
	} else {
		return fmt.Errorf("container.isolatedContainerContext is not set to launch isolated container")
	}

	go func(ctr *container) {
		active := true

		// Wait for few seconds so that VM does not start, the isolated container should not be shown as running.
		time.Sleep(3 * time.Second)

		domain := ctr.isolatedContainerContext.GetDomain()

		if domain!= nil {
			for active {
				active, err = domain.IsActive()
				if err != nil {
					logrus.Error("Could not check if the domain is active: ", err)
					active = false
				}
				// Do Nothing
			}
		}

		if err == nil {
			logrus.Infof("The isolated container %s is no more running", ctr.containerID)
			ctr.client.backend.StateChanged(ctr.containerID, StateInfo{
				CommonStateInfo: CommonStateInfo{
					State: StateExit,
			}})
		}
	}(ctr)

	return ctr.client.backend.StateChanged(ctr.containerID, StateInfo{
		CommonStateInfo: CommonStateInfo{
			State: StateStart,
		}})
}

func (ctr *container) start(checkpoint string, checkpointDir string, attachStdio StdioCallback) error {
	spec, err := ctr.spec()
	if err != nil {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})

	iopipe, err := ctr.openFifos(spec.Process.Terminal)
	if err != nil {
		return err
	}

	var stdinOnce sync.Once

	// we need to delay stdin closure after container start or else "stdin close"
	// event will be rejected by containerd.
	// stdin closure happens in attachStdio
	stdin := iopipe.Stdin
	iopipe.Stdin = ioutils.NewWriteCloserWrapper(stdin, func() error {
		var err error
		stdinOnce.Do(func() { // on error from attach we don't know if stdin was already closed
			err = stdin.Close()
			go func() {
				select {
				case <-ready:
					if err := ctr.sendCloseStdin(); err != nil {
						logrus.Warnf("failed to close stdin: %+v", err)
					}
				case <-ctx.Done():
				}
			}()
		})
		return err
	})

	r := &containerd.CreateContainerRequest{
		Id:            ctr.containerID,
		BundlePath:    ctr.dir,
		Stdin:         ctr.fifo(syscall.Stdin),
		Stdout:        ctr.fifo(syscall.Stdout),
		Stderr:        ctr.fifo(syscall.Stderr),
		Checkpoint:    checkpoint,
		CheckpointDir: checkpointDir,
		// check to see if we are running in ramdisk to disable pivot root
		NoPivotRoot: os.Getenv("DOCKER_RAMDISK") != "",
		Runtime:     ctr.runtime,
		RuntimeArgs: ctr.runtimeArgs,
	}
	ctr.client.appendContainer(ctr)

	if err := attachStdio(*iopipe); err != nil {
		ctr.closeFifos(iopipe)
		return err
	}

	resp, err := ctr.client.remote.apiClient.CreateContainer(context.Background(), r)
	if err != nil {
		ctr.closeFifos(iopipe)
		return err
	}
	ctr.systemPid = systemPid(resp.Container)
	close(ready)

	return ctr.client.backend.StateChanged(ctr.containerID, StateInfo{
		CommonStateInfo: CommonStateInfo{
			State: StateStart,
			Pid:   ctr.systemPid,
		}})
}

func (ctr *container) newProcess(friendlyName string) *process {
	return &process{
		dir: ctr.dir,
		processCommon: processCommon{
			containerID:  ctr.containerID,
			friendlyName: friendlyName,
			client:       ctr.client,
		},
	}
}

func (ctr *container) handleEvent(e *containerd.Event) error {
	ctr.client.lock(ctr.containerID)
	defer ctr.client.unlock(ctr.containerID)
	switch e.Type {
	case StateExit, StatePause, StateResume, StateOOM:
		st := StateInfo{
			CommonStateInfo: CommonStateInfo{
				State:    e.Type,
				ExitCode: e.Status,
			},
			OOMKilled: e.Type == StateExit && ctr.oom,
		}
		if e.Type == StateOOM {
			ctr.oom = true
		}
		if e.Type == StateExit && e.Pid != InitFriendlyName {
			st.ProcessID = e.Pid
			st.State = StateExitProcess
		}

		// Remove process from list if we have exited
		switch st.State {
		case StateExit:
			ctr.clean()
			ctr.client.deleteContainer(e.Id)
		case StateExitProcess:
			ctr.cleanProcess(st.ProcessID)
		}
		ctr.client.q.append(e.Id, func() {
			if err := ctr.client.backend.StateChanged(e.Id, st); err != nil {
				logrus.Errorf("libcontainerd: backend.StateChanged(): %v", err)
			}
			if e.Type == StatePause || e.Type == StateResume {
				ctr.pauseMonitor.handle(e.Type)
			}
			if e.Type == StateExit {
				if en := ctr.client.getExitNotifier(e.Id); en != nil {
					en.close()
				}
			}
		})

	default:
		logrus.Debugf("libcontainerd: event unhandled: %+v", e)
	}
	return nil
}

// discardFifos attempts to fully read the container fifos to unblock processes
// that may be blocked on the writer side.
func (ctr *container) discardFifos() {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	for _, i := range []int{syscall.Stdout, syscall.Stderr} {
		f, err := fifo.OpenFifo(ctx, ctr.fifo(i), syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			logrus.Warnf("error opening fifo %v for discarding: %+v", f, err)
			continue
		}
		go func() {
			io.Copy(ioutil.Discard, f)
		}()
	}
}
