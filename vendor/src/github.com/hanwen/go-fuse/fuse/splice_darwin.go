package fuse

import (
	"fmt"
)

func (s *Server) setSplice() {
	panic("darwin has no splice.")
}

func (ms *Server) trySplice(header []byte, req *request, fdData *readResultFd) error {
	return fmt.Errorf("unimplemented")
}
