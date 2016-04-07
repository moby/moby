package libcontainerd

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Sirupsen/logrus"
	containerd "github.com/docker/containerd/api/grpc/types"
	"github.com/opencontainers/specs/specs-go"
	"golang.org/x/net/context"
)

type container struct {
	containerCommon

	// Platform specific fields are below here.
	pauseMonitor
	oom bool
}

func (ctr *container) clean() error {
	if _, err := os.Lstat(ctr.dir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	syscall.Unmount(filepath.Join(ctr.dir, "rootfs"), syscall.MNT_DETACH) // ignore error
	if err := os.RemoveAll(ctr.dir); err != nil {
		return err
	}
	return nil
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

func (ctr *container) start() error {
	spec, err := ctr.spec()
	if err != nil {
		return nil
	}
	iopipe, err := ctr.openFifos(spec.Process.Terminal)
	if err != nil {
		return err
	}

	r := &containerd.CreateContainerRequest{
		Id:         ctr.containerID,
		BundlePath: ctr.dir,
		Stdin:      ctr.fifo(syscall.Stdin),
		Stdout:     ctr.fifo(syscall.Stdout),
		Stderr:     ctr.fifo(syscall.Stderr),
		// check to see if we are running in ramdisk to disable pivot root
		NoPivotRoot: os.Getenv("DOCKER_RAMDISK") != "",
	}
	ctr.client.appendContainer(ctr)

	resp, err := ctr.client.remote.apiClient.CreateContainer(context.Background(), r)
	if err != nil {
		ctr.closeFifos(iopipe)
		return err
	}

	if err := ctr.client.backend.AttachStreams(ctr.containerID, *iopipe); err != nil {
		return err
	}
	ctr.systemPid = systemPid(resp.Container)

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
		if st.State == StateExit && ctr.restartManager != nil {
			restart, wait, err := ctr.restartManager.ShouldRestart(e.Status)
			if err != nil {
				logrus.Error(err)
			} else if restart {
				st.State = StateRestart
				ctr.restarting = true
				ctr.client.deleteContainer(e.Id)
				go func() {
					err := <-wait
					ctr.restarting = false
					if err != nil {
						st.State = StateExit
						ctr.client.q.append(e.Id, func() {
							if err := ctr.client.backend.StateChanged(e.Id, st); err != nil {
								logrus.Error(err)
							}
						})
						logrus.Error(err)
					} else {
						ctr.start()
					}
				}()
			}
		}

		// Remove process from list if we have exited
		// We need to do so here in case the Message Handler decides to restart it.
		if st.State == StateExit {
			if os.Getenv("LIBCONTAINERD_NOCLEAN") != "1" {
				ctr.clean()
			}
			ctr.client.deleteContainer(e.Id)
		}
		ctr.client.q.append(e.Id, func() {
			if err := ctr.client.backend.StateChanged(e.Id, st); err != nil {
				logrus.Error(err)
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
		logrus.Debugf("event unhandled: %+v", e)
	}
	return nil
}
