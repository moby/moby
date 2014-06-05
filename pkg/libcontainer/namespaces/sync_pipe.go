package namespaces

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/dotcloud/docker/pkg/libcontainer"
)

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

func (s *SyncPipe) SendToChild(context libcontainer.Context) error {
	data, err := json.Marshal(context)
	if err != nil {
		return err
	}
	s.parent.Write(data)
	return nil
}

func (s *SyncPipe) ReadFromParent() (libcontainer.Context, error) {
	data, err := ioutil.ReadAll(s.child)
	if err != nil {
		return nil, fmt.Errorf("error reading from sync pipe %s", err)
	}
	var context libcontainer.Context
	if len(data) > 0 {
		if err := json.Unmarshal(data, &context); err != nil {
			return nil, err
		}
	}
	return context, nil

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
