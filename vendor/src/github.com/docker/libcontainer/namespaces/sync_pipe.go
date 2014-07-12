package namespaces

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"syscall"

	"github.com/docker/libcontainer/network"
)

// SyncPipe allows communication to and from the child processes
// to it's parent and allows the two independent processes to
// syncronize their state.
type SyncPipe struct {
	parent, child *os.File
}

func NewSyncPipeFromFd(parentFd, childFd uintptr) (*SyncPipe, error) {
	s := &SyncPipe{}

	if parentFd > 0 {
		s.parent = os.NewFile(parentFd, "parentPipe")
	} else if childFd > 0 {
		s.child = os.NewFile(childFd, "childPipe")
	} else {
		return nil, fmt.Errorf("no valid sync pipe fd specified")
	}

	return s, nil
}

func (s *SyncPipe) Child() *os.File {
	return s.child
}

func (s *SyncPipe) Parent() *os.File {
	return s.parent
}

func (s *SyncPipe) SendToChild(networkState *network.NetworkState) error {
	data, err := json.Marshal(networkState)
	if err != nil {
		return err
	}

	s.parent.Write(data)

	return syscall.Shutdown(int(s.parent.Fd()), syscall.SHUT_WR)
}

func (s *SyncPipe) ReadFromChild() error {
	data, err := ioutil.ReadAll(s.parent)
	if err != nil {
		return err
	}

	if len(data) > 0 {
		return fmt.Errorf("%s", data)
	}

	return nil
}

func (s *SyncPipe) ReadFromParent() (*network.NetworkState, error) {
	data, err := ioutil.ReadAll(s.child)
	if err != nil {
		return nil, fmt.Errorf("error reading from sync pipe %s", err)
	}
	var networkState *network.NetworkState
	if len(data) > 0 {
		if err := json.Unmarshal(data, &networkState); err != nil {
			return nil, err
		}
	}
	return networkState, nil
}

func (s *SyncPipe) ReportChildError(err error) {
	s.child.Write([]byte(err.Error()))
	s.CloseChild()
}

func (s *SyncPipe) Close() error {
	if s.parent != nil {
		s.parent.Close()
	}

	if s.child != nil {
		s.child.Close()
	}

	return nil
}

func (s *SyncPipe) CloseChild() {
	if s.child != nil {
		s.child.Close()
		s.child = nil
	}
}
