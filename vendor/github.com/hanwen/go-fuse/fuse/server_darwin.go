// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"syscall"
)

func (ms *Server) systemWrite(req *request, header []byte) Status {
	if req.flatDataSize() == 0 {
		err := handleEINTR(func() error {
			_, err := syscall.Write(ms.mountFd, header)
			return err
		})
		return ToStatus(err)
	}

	if req.fdData != nil {
		sz := req.flatDataSize()
		buf := ms.allocOut(req, uint32(sz))
		req.flatData, req.status = req.fdData.Bytes(buf)
		header = req.serializeHeader(len(req.flatData))
	}

	_, err := writev(int(ms.mountFd), [][]byte{header, req.flatData})
	if req.readResult != nil {
		req.readResult.Done()
	}
	return ToStatus(err)
}
