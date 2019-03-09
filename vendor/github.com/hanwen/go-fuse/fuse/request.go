// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"
	"unsafe"
)

var sizeOfOutHeader = unsafe.Sizeof(OutHeader{})
var zeroOutBuf [outputHeaderSize]byte

type request struct {
	inputBuf []byte

	// These split up inputBuf.
	inHeader *InHeader      // generic header
	inData   unsafe.Pointer // per op data
	arg      []byte         // flat data.

	filenames []string // filename arguments

	// Output data.
	status   Status
	flatData []byte
	fdData   *readResultFd

	// In case of read, keep read result here so we can call
	// Done() on it.
	readResult ReadResult

	// Start timestamp for timing info.
	startTime time.Time

	// All information pertaining to opcode of this request.
	handler *operationHandler

	// Request storage. For large inputs and outputs, use data
	// obtained through bufferpool.
	bufferPoolInputBuf  []byte
	bufferPoolOutputBuf []byte

	// For small pieces of data, we use the following inlines
	// arrays:
	//
	// Output header and structured data.
	outBuf [outputHeaderSize]byte

	// Input, if small enough to fit here.
	smallInputBuf [128]byte

	context Context
}

func (r *request) clear() {
	r.inputBuf = nil
	r.inHeader = nil
	r.inData = nil
	r.arg = nil
	r.filenames = nil
	r.status = OK
	r.flatData = nil
	r.fdData = nil
	r.startTime = time.Time{}
	r.handler = nil
	r.readResult = nil
}

func (r *request) InputDebug() string {
	val := ""
	if r.handler.DecodeIn != nil {
		val = fmt.Sprintf("%v ", Print(r.handler.DecodeIn(r.inData)))
	}

	names := ""
	if r.filenames != nil {
		names = fmt.Sprintf("%q", r.filenames)
	}

	if len(r.arg) > 0 {
		names += fmt.Sprintf(" %db", len(r.arg))
	}

	return fmt.Sprintf("rx %d: %s i%d %s%s",
		r.inHeader.Unique, operationName(r.inHeader.Opcode), r.inHeader.NodeId,
		val, names)
}

func (r *request) OutputDebug() string {
	var dataStr string
	if r.handler.DecodeOut != nil && r.handler.OutputSize > 0 {
		dataStr = Print(r.handler.DecodeOut(r.outData()))
	}

	max := 1024
	if len(dataStr) > max {
		dataStr = dataStr[:max] + fmt.Sprintf(" ...trimmed")
	}

	flatStr := ""
	if r.flatDataSize() > 0 {
		if r.handler.FileNameOut {
			s := strings.TrimRight(string(r.flatData), "\x00")
			flatStr = fmt.Sprintf(" %q", s)
		} else {
			spl := ""
			if r.fdData != nil {
				spl = " (fd data)"
			} else {
				l := len(r.flatData)
				s := ""
				if l > 8 {
					l = 8
					s = "..."
				}
				spl = fmt.Sprintf(" %q%s", r.flatData[:l], s)
			}
			flatStr = fmt.Sprintf(" %db data%s", r.flatDataSize(), spl)
		}
	}

	extraStr := dataStr + flatStr
	if extraStr != "" {
		extraStr = ", " + extraStr
	}
	return fmt.Sprintf("tx %d:     %v%s",
		r.inHeader.Unique, r.status, extraStr)
}

// setInput returns true if it takes ownership of the argument, false if not.
func (r *request) setInput(input []byte) bool {
	if len(input) < len(r.smallInputBuf) {
		copy(r.smallInputBuf[:], input)
		r.inputBuf = r.smallInputBuf[:len(input)]
		return false
	}
	r.inputBuf = input
	r.bufferPoolInputBuf = input[:cap(input)]

	return true
}

func (r *request) parse() {
	inHSize := int(unsafe.Sizeof(InHeader{}))
	if len(r.inputBuf) < inHSize {
		log.Printf("Short read for input header: %v", r.inputBuf)
		return
	}

	r.inHeader = (*InHeader)(unsafe.Pointer(&r.inputBuf[0]))
	r.arg = r.inputBuf[:]

	r.handler = getHandler(r.inHeader.Opcode)
	if r.handler == nil {
		log.Printf("Unknown opcode %d", r.inHeader.Opcode)
		r.status = ENOSYS
		return
	}

	if len(r.arg) < int(r.handler.InputSize) {
		log.Printf("Short read for %v: %v", operationName(r.inHeader.Opcode), r.arg)
		r.status = EIO
		return
	}

	if r.handler.InputSize > 0 {
		r.inData = unsafe.Pointer(&r.arg[0])
		r.arg = r.arg[r.handler.InputSize:]
	} else {
		r.arg = r.arg[inHSize:]
	}

	count := r.handler.FileNames
	if count > 0 {
		if count == 1 && r.inHeader.Opcode == _OP_SETXATTR {
			// SETXATTR is special: the only opcode with a file name AND a
			// binary argument.
			splits := bytes.SplitN(r.arg, []byte{0}, 2)
			r.filenames = []string{string(splits[0])}
		} else if count == 1 {
			r.filenames = []string{string(r.arg[:len(r.arg)-1])}
		} else {
			names := bytes.SplitN(r.arg[:len(r.arg)-1], []byte{0}, count)
			r.filenames = make([]string, len(names))
			for i, n := range names {
				r.filenames[i] = string(n)
			}
			if len(names) != count {
				log.Println("filename argument mismatch", names, count)
				r.status = EIO
			}
		}
	}

	copy(r.outBuf[:r.handler.OutputSize+sizeOfOutHeader],
		zeroOutBuf[:r.handler.OutputSize+sizeOfOutHeader])
}

func (r *request) outData() unsafe.Pointer {
	return unsafe.Pointer(&r.outBuf[sizeOfOutHeader])
}

// serializeHeader serializes the response header. The header points
// to an internal buffer of the receiver.
func (r *request) serializeHeader(flatDataSize int) (header []byte) {
	dataLength := r.handler.OutputSize
	if r.status > OK {
		dataLength = 0
	}

	// [GET|LIST]XATTR is two opcodes in one: get/list xattr size (return
	// structured GetXAttrOut, no flat data) and get/list xattr data
	// (return no structured data, but only flat data)
	if r.inHeader.Opcode == _OP_GETXATTR || r.inHeader.Opcode == _OP_LISTXATTR {
		if (*GetXAttrIn)(r.inData).Size != 0 {
			dataLength = 0
		}
	}

	header = r.outBuf[:sizeOfOutHeader+dataLength]
	o := (*OutHeader)(unsafe.Pointer(&header[0]))
	o.Unique = r.inHeader.Unique
	o.Status = int32(-r.status)
	o.Length = uint32(
		int(sizeOfOutHeader) + int(dataLength) + flatDataSize)
	return header
}

func (r *request) flatDataSize() int {
	if r.fdData != nil {
		return r.fdData.Size()
	}
	return len(r.flatData)
}
