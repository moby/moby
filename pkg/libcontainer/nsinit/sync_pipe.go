package nsinit

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/dotcloud/docker/pkg/libcontainer"
)

type SyncPipeData struct {
	Context libcontainer.Context
	Files   map[string][]byte
}

// SyncPipe allows communication to and from the child processes
// to it's parent and allows the two independent processes to
// syncronize their state.
type SyncPipe struct {
	parent, child *os.File
}

func NewSyncPipe() (s *SyncPipe, err error) {
	s = &SyncPipe{}
	s.child, s.parent, err = os.Pipe()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func NewSyncPipeFromFd(parendFd, childFd uintptr) (*SyncPipe, error) {
	s := &SyncPipe{}
	if parendFd > 0 {
		s.parent = os.NewFile(parendFd, "parendPipe")
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

func (s *SyncPipe) SendToChild(pipeData *SyncPipeData) error {
	data, err := json.Marshal(pipeData)
	if err != nil {
		return err
	}
	s.parent.Write(data)
	return nil
}

func (s *SyncPipe) ReadFromParent() (*SyncPipeData, error) {
	data, err := ioutil.ReadAll(s.child)
	if err != nil {
		return nil, fmt.Errorf("error reading from sync pipe %s", err)
	}
	var pipeData SyncPipeData
	if len(data) > 0 {
		if err := json.Unmarshal(data, &pipeData); err != nil {
			return nil, err
		}
	}
	return &pipeData, nil

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
