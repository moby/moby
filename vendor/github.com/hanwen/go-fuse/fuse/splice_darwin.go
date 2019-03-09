// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"fmt"
)

func (s *Server) setSplice() {
	s.canSplice = false
}

func (ms *Server) trySplice(header []byte, req *request, fdData *readResultFd) error {
	return fmt.Errorf("unimplemented")
}
