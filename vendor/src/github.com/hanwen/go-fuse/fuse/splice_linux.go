package fuse

import (
	"fmt"
	"os"

	"github.com/hanwen/go-fuse/splice"
)

func (s *Server) setSplice() {
	s.canSplice = splice.Resizable()
}

// trySplice:  Zero-copy read from fdData.Fd into /dev/fuse
//
// This is a four-step process:
//
//	1) Splice data form fdData.Fd into the "pair1" pipe buffer --> pair1: [payload]
//	   Now we know the actual payload length and can
//     construct the reply header
//	2) Write header into the "pair2" pipe buffer               --> pair2: [header]
//	4) Splice data from "pair1" into "pair2"                   --> pair2: [header][payload]
//	3) Splice the data from "pair2" into /dev/fuse
//
// This dance is neccessary because header and payload cannot be split across
// two splices and we cannot seek in a pipe buffer.
func (ms *Server) trySplice(header []byte, req *request, fdData *readResultFd) error {
	var err error

	// Get a pair of connected pipes
	pair1, err := splice.Get()
	if err != nil {
		return err
	}
	defer splice.Done(pair1)

	// Grow buffer pipe to requested size + one extra page
	// Without the extra page the kernel will block once the pipe is almost full
	pair1Sz := fdData.Size() + os.Getpagesize()
	if err := pair1.Grow(pair1Sz); err != nil {
		return err
	}

	// Read data from file
	payloadLen, err := pair1.LoadFromAt(fdData.Fd, fdData.Size(), fdData.Off)

	if err != nil {
		// TODO - extract the data from splice.
		return err
	}

	// Get another pair of connected pipes
	pair2, err := splice.Get()
	if err != nil {
		return err
	}
	defer splice.Done(pair2)

	// Grow pipe to header + actually read size + one extra page
	// Without the extra page the kernel will block once the pipe is almost full
	header = req.serializeHeader(payloadLen)
	total := len(header) + payloadLen
	pair2Sz := total + os.Getpagesize()
	if err := pair2.Grow(pair2Sz); err != nil {
		return err
	}

	// Write header into pair2
	n, err := pair2.Write(header)
	if err != nil {
		return err
	}
	if n != len(header) {
		return fmt.Errorf("Short write into splice: wrote %d, want %d", n, len(header))
	}

	// Write data into pair2
	n, err = pair2.LoadFrom(pair1.ReadFd(), payloadLen)
	if err != nil {
		return err
	}
	if n != payloadLen {
		return fmt.Errorf("Short splice: wrote %d, want %d", n, payloadLen)
	}

	// Write header + data to /dev/fuse
	_, err = pair2.WriteTo(uintptr(ms.mountFd), total)
	if err != nil {
		return err
	}

	return nil
}
