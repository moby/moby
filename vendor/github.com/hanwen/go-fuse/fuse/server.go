// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// The kernel caps writes at 128k.
	MAX_KERNEL_WRITE = 128 * 1024
)

// Server contains the logic for reading from the FUSE device and
// translating it to RawFileSystem interface calls.
type Server struct {
	// Empty if unmounted.
	mountPoint string
	fileSystem RawFileSystem

	// writeMu serializes close and notify writes
	writeMu sync.Mutex

	// I/O with kernel and daemon.
	mountFd int

	latencies LatencyMap

	opts *MountOptions

	// Pool for request structs.
	reqPool sync.Pool

	// Pool for raw requests data
	readPool       sync.Pool
	reqMu          sync.Mutex
	reqReaders     int
	kernelSettings InitIn

	// in-flight notify-retrieve queries
	retrieveMu   sync.Mutex
	retrieveNext uint64
	retrieveTab  map[uint64]*retrieveCacheRequest // notifyUnique -> retrieve request

	singleReader bool
	canSplice    bool
	loops        sync.WaitGroup

	ready chan error
}

// SetDebug is deprecated. Use MountOptions.Debug instead.
func (ms *Server) SetDebug(dbg bool) {
	// This will typically trigger the race detector.
	ms.opts.Debug = dbg
}

// KernelSettings returns the Init message from the kernel, so
// filesystems can adapt to availability of features of the kernel
// driver. The message should not be altered.
func (ms *Server) KernelSettings() *InitIn {
	ms.reqMu.Lock()
	s := ms.kernelSettings
	ms.reqMu.Unlock()

	return &s
}

const _MAX_NAME_LEN = 20

// This type may be provided for recording latencies of each FUSE
// operation.
type LatencyMap interface {
	Add(name string, dt time.Duration)
}

// RecordLatencies switches on collection of timing for each request
// coming from the kernel.P assing a nil argument switches off the
func (ms *Server) RecordLatencies(l LatencyMap) {
	ms.latencies = l
}

// Unmount calls fusermount -u on the mount. This has the effect of
// shutting down the filesystem. After the Server is unmounted, it
// should be discarded.
func (ms *Server) Unmount() (err error) {
	if ms.mountPoint == "" {
		return nil
	}
	delay := time.Duration(0)
	for try := 0; try < 5; try++ {
		err = unmount(ms.mountPoint)
		if err == nil {
			break
		}

		// Sleep for a bit. This is not pretty, but there is
		// no way we can be certain that the kernel thinks all
		// open files have already been closed.
		delay = 2*delay + 5*time.Millisecond
		time.Sleep(delay)
	}
	if err != nil {
		return
	}
	// Wait for event loops to exit.
	ms.loops.Wait()
	ms.mountPoint = ""
	return err
}

// NewServer creates a server and attaches it to the given directory.
func NewServer(fs RawFileSystem, mountPoint string, opts *MountOptions) (*Server, error) {
	if opts == nil {
		opts = &MountOptions{
			MaxBackground: _DEFAULT_BACKGROUND_TASKS,
		}
	}
	o := *opts
	if o.SingleThreaded {
		fs = NewLockingRawFileSystem(fs)
	}

	if o.Buffers == nil {
		o.Buffers = defaultBufferPool
	}
	if o.MaxWrite < 0 {
		o.MaxWrite = 0
	}
	if o.MaxWrite == 0 {
		o.MaxWrite = 1 << 16
	}
	if o.MaxWrite > MAX_KERNEL_WRITE {
		o.MaxWrite = MAX_KERNEL_WRITE
	}
	if o.Name == "" {
		name := fs.String()
		l := len(name)
		if l > _MAX_NAME_LEN {
			l = _MAX_NAME_LEN
		}
		o.Name = strings.Replace(name[:l], ",", ";", -1)
	}

	for _, s := range o.optionsStrings() {
		if strings.Contains(s, ",") {
			return nil, fmt.Errorf("found ',' in option string %q", s)
		}
	}

	ms := &Server{
		fileSystem:  fs,
		opts:        &o,
		retrieveTab: make(map[uint64]*retrieveCacheRequest),
		// OSX has races when multiple routines read from the
		// FUSE device: on unmount, sometime some reads do not
		// error-out, meaning that unmount will hang.
		singleReader: runtime.GOOS == "darwin",
		ready:        make(chan error, 1),
	}
	ms.reqPool.New = func() interface{} { return new(request) }
	ms.readPool.New = func() interface{} { return make([]byte, o.MaxWrite+pageSize) }

	mountPoint = filepath.Clean(mountPoint)
	if !filepath.IsAbs(mountPoint) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		mountPoint = filepath.Clean(filepath.Join(cwd, mountPoint))
	}
	fd, err := mount(mountPoint, &o, ms.ready)
	if err != nil {
		return nil, err
	}

	ms.mountPoint = mountPoint
	ms.mountFd = fd

	if code := ms.handleInit(); !code.Ok() {
		syscall.Close(fd)
		// TODO - unmount as well?
		return nil, fmt.Errorf("init: %s", code)
	}
	return ms, nil
}

func (o *MountOptions) optionsStrings() []string {
	var r []string
	r = append(r, o.Options...)

	if o.AllowOther {
		r = append(r, "allow_other")
	}

	if o.FsName != "" {
		r = append(r, "fsname="+o.FsName)
	}
	if o.Name != "" {
		r = append(r, "subtype="+o.Name)
	}

	return r
}

// DebugData returns internal status information for debugging
// purposes.
func (ms *Server) DebugData() string {
	var r int
	ms.reqMu.Lock()
	r = ms.reqReaders
	ms.reqMu.Unlock()

	return fmt.Sprintf("readers: %d", r)
}

// What is a good number?  Maybe the number of CPUs?
const _MAX_READERS = 2

// handleEINTR retries the given function until it doesn't return syscall.EINTR.
// This is similar to the HANDLE_EINTR() macro from Chromium ( see
// https://code.google.com/p/chromium/codesearch#chromium/src/base/posix/eintr_wrapper.h
// ) and the TEMP_FAILURE_RETRY() from glibc (see
// https://www.gnu.org/software/libc/manual/html_node/Interrupted-Primitives.html
// ).
//
// Don't use handleEINTR() with syscall.Close(); see
// https://code.google.com/p/chromium/issues/detail?id=269623 .
func handleEINTR(fn func() error) (err error) {
	for {
		err = fn()
		if err != syscall.EINTR {
			break
		}
	}
	return
}

// Returns a new request, or error. In case exitIdle is given, returns
// nil, OK if we have too many readers already.
func (ms *Server) readRequest(exitIdle bool) (req *request, code Status) {
	ms.reqMu.Lock()
	if ms.reqReaders > _MAX_READERS {
		ms.reqMu.Unlock()
		return nil, OK
	}
	req = ms.reqPool.Get().(*request)
	dest := ms.readPool.Get().([]byte)
	ms.reqReaders++
	ms.reqMu.Unlock()

	var n int
	err := handleEINTR(func() error {
		var err error
		n, err = syscall.Read(ms.mountFd, dest)
		return err
	})
	if err != nil {
		code = ToStatus(err)
		ms.reqPool.Put(req)
		ms.reqMu.Lock()
		ms.reqReaders--
		ms.reqMu.Unlock()
		return nil, code
	}

	if ms.latencies != nil {
		req.startTime = time.Now()
	}
	gobbled := req.setInput(dest[:n])

	ms.reqMu.Lock()
	if !gobbled {
		ms.readPool.Put(dest)
		dest = nil
	}
	ms.reqReaders--
	if !ms.singleReader && ms.reqReaders <= 0 {
		ms.loops.Add(1)
		go ms.loop(true)
	}
	ms.reqMu.Unlock()

	return req, OK
}

// returnRequest returns a request to the pool of unused requests.
func (ms *Server) returnRequest(req *request) {
	ms.recordStats(req)

	if req.bufferPoolOutputBuf != nil {
		ms.opts.Buffers.FreeBuffer(req.bufferPoolOutputBuf)
		req.bufferPoolOutputBuf = nil
	}

	req.clear()

	if p := req.bufferPoolInputBuf; p != nil {
		req.bufferPoolInputBuf = nil
		ms.readPool.Put(p)
	}
	ms.reqPool.Put(req)
}

func (ms *Server) recordStats(req *request) {
	if ms.latencies != nil {
		dt := time.Now().Sub(req.startTime)
		opname := operationName(req.inHeader.Opcode)
		ms.latencies.Add(opname, dt)
	}
}

// Serve initiates the FUSE loop. Normally, callers should run Serve()
// and wait for it to exit, but tests will want to run this in a
// goroutine.
//
// Each filesystem operation executes in a separate goroutine.
func (ms *Server) Serve() {
	ms.loops.Add(1)
	ms.loop(false)
	ms.loops.Wait()

	ms.writeMu.Lock()
	syscall.Close(ms.mountFd)
	ms.writeMu.Unlock()

	// shutdown in-flight cache retrieves.
	//
	// It is possible that umount comes in the middle - after retrieve
	// request was sent to kernel, but corresponding kernel reply has not
	// yet been read. We unblock all such readers and wake them up with ENODEV.
	ms.retrieveMu.Lock()
	rtab := ms.retrieveTab
	// retrieve attempts might be erroneously tried even after close
	// we have to keep retrieveTab !nil not to panic.
	ms.retrieveTab = make(map[uint64]*retrieveCacheRequest)
	ms.retrieveMu.Unlock()
	for _, reading := range rtab {
		reading.n = 0
		reading.st = ENODEV
		close(reading.ready)
	}
}

func (ms *Server) handleInit() Status {
	// The first request should be INIT; read it synchronously,
	// and don't spawn new readers.
	orig := ms.singleReader
	ms.singleReader = true
	req, errNo := ms.readRequest(false)
	ms.singleReader = orig

	if errNo != OK || req == nil {
		return errNo
	}
	if code := ms.handleRequest(req); !code.Ok() {
		return code
	}

	// INIT is handled. Init the file system, but don't accept
	// incoming requests, so the file system can setup itself.
	ms.fileSystem.Init(ms)
	return OK
}

func (ms *Server) loop(exitIdle bool) {
	defer ms.loops.Done()
exit:
	for {
		req, errNo := ms.readRequest(exitIdle)
		switch errNo {
		case OK:
			if req == nil {
				break exit
			}
		case ENOENT:
			continue
		case ENODEV:
			// unmount
			if ms.opts.Debug {
				log.Printf("received ENODEV (unmount request), thread exiting")
			}
			break exit
		default: // some other error?
			log.Printf("Failed to read from fuse conn: %v", errNo)
			break exit
		}

		if ms.singleReader {
			go ms.handleRequest(req)
		} else {
			ms.handleRequest(req)
		}
	}
}

func (ms *Server) handleRequest(req *request) Status {
	req.parse()
	if req.handler == nil {
		req.status = ENOSYS
	}

	if req.status.Ok() && ms.opts.Debug {
		log.Println(req.InputDebug())
	}

	if req.inHeader.NodeId == pollHackInode {
		// We want to avoid switching off features through our
		// poll hack, so don't use ENOSYS
		req.status = EIO
		if req.inHeader.Opcode == _OP_POLL {
			req.status = ENOSYS
		}
	} else if req.inHeader.NodeId == FUSE_ROOT_ID && len(req.filenames) > 0 && req.filenames[0] == pollHackName {
		doPollHackLookup(ms, req)
	} else if req.status.Ok() && req.handler.Func == nil {
		log.Printf("Unimplemented opcode %v", operationName(req.inHeader.Opcode))
		req.status = ENOSYS
	} else if req.status.Ok() {
		req.handler.Func(ms, req)
	}

	errNo := ms.write(req)
	if errNo != 0 {
		log.Printf("writer: Write/Writev failed, err: %v. opcode: %v",
			errNo, operationName(req.inHeader.Opcode))
	}
	ms.returnRequest(req)
	return Status(errNo)
}

func (ms *Server) allocOut(req *request, size uint32) []byte {
	if cap(req.bufferPoolOutputBuf) >= int(size) {
		req.bufferPoolOutputBuf = req.bufferPoolOutputBuf[:size]
		return req.bufferPoolOutputBuf
	}
	if req.bufferPoolOutputBuf != nil {
		ms.opts.Buffers.FreeBuffer(req.bufferPoolOutputBuf)
	}
	req.bufferPoolOutputBuf = ms.opts.Buffers.AllocBuffer(size)
	return req.bufferPoolOutputBuf
}

func (ms *Server) write(req *request) Status {
	// Forget/NotifyReply do not wait for reply from filesystem server.
	switch req.inHeader.Opcode {
	case _OP_FORGET, _OP_BATCH_FORGET, _OP_NOTIFY_REPLY:
		return OK
	}

	header := req.serializeHeader(req.flatDataSize())
	if ms.opts.Debug {
		log.Println(req.OutputDebug())
	}

	if header == nil {
		return OK
	}

	s := ms.systemWrite(req, header)
	return s
}

// InodeNotify invalidates the information associated with the inode
// (ie. data cache, attributes, etc.)
func (ms *Server) InodeNotify(node uint64, off int64, length int64) Status {
	if !ms.kernelSettings.SupportsNotify(NOTIFY_INVAL_INODE) {
		return ENOSYS
	}

	req := request{
		inHeader: &InHeader{
			Opcode: _OP_NOTIFY_INVAL_INODE,
		},
		handler: operationHandlers[_OP_NOTIFY_INVAL_INODE],
		status:  NOTIFY_INVAL_INODE,
	}

	entry := (*NotifyInvalInodeOut)(req.outData())
	entry.Ino = node
	entry.Off = off
	entry.Length = length

	// Protect against concurrent close.
	ms.writeMu.Lock()
	result := ms.write(&req)
	ms.writeMu.Unlock()

	if ms.opts.Debug {
		log.Println("Response: INODE_NOTIFY", result)
	}
	return result
}

// InodeNotifyStoreCache tells kernel to store data into inode's cache.
//
// This call is similar to InodeNotify, but instead of only invalidating a data
// region, it gives updated data directly to the kernel.
func (ms *Server) InodeNotifyStoreCache(node uint64, offset int64, data []byte) Status {
	if !ms.kernelSettings.SupportsNotify(NOTIFY_STORE_CACHE) {
		return ENOSYS
	}

	for len(data) > 0 {
		size := len(data)
		if size > math.MaxInt32 {
			// NotifyStoreOut has only uint32 for size.
			// we check for max(int32), not max(uint32), because on 32-bit
			// platforms int has only 31-bit for positive range.
			size = math.MaxInt32
		}

		st := ms.inodeNotifyStoreCache32(node, offset, data[:size])
		if st != OK {
			return st
		}

		data = data[size:]
		offset += int64(size)
	}

	return OK
}

// inodeNotifyStoreCache32 is internal worker for InodeNotifyStoreCache which
// handles data chunks not larger than 2GB.
func (ms *Server) inodeNotifyStoreCache32(node uint64, offset int64, data []byte) Status {
	req := request{
		inHeader: &InHeader{
			Opcode: _OP_NOTIFY_STORE_CACHE,
		},
		handler: operationHandlers[_OP_NOTIFY_STORE_CACHE],
		status:  NOTIFY_STORE_CACHE,
	}

	store := (*NotifyStoreOut)(req.outData())
	store.Nodeid = node
	store.Offset = uint64(offset) // NOTE not int64, as it is e.g. in NotifyInvalInodeOut
	store.Size = uint32(len(data))

	req.flatData = data

	// Protect against concurrent close.
	ms.writeMu.Lock()
	result := ms.write(&req)
	ms.writeMu.Unlock()

	if ms.opts.Debug {
		log.Printf("Response: INODE_NOTIFY_STORE_CACHE: %v", result)
	}
	return result
}

// InodeRetrieveCache retrieves data from kernel's inode cache.
//
// InodeRetrieveCache asks kernel to return data from its cache for inode at
// [offset:offset+len(dest)) and waits for corresponding reply. If kernel cache
// has fewer consecutive data starting at offset, that fewer amount is returned.
// In particular if inode data at offset is not cached (0, OK) is returned.
//
// The kernel returns ENOENT if it does not currently have entry for this inode
// in its dentry cache.
func (ms *Server) InodeRetrieveCache(node uint64, offset int64, dest []byte) (n int, st Status) {
	// the kernel won't send us in one go more then what we negotiated as MaxWrite.
	// retrieve the data in chunks.
	// TODO spawn some number of readahead retrievers in parallel.
	ntotal := 0
	for {
		chunkSize := len(dest)
		if chunkSize > ms.opts.MaxWrite {
			chunkSize = ms.opts.MaxWrite
		}
		n, st = ms.inodeRetrieveCache1(node, offset, dest[:chunkSize])
		if st != OK || n == 0 {
			break
		}

		ntotal += n
		offset += int64(n)
		dest = dest[n:]
	}

	// if we could retrieve at least something - it is ok.
	// if ntotal=0 - st will be st returned from first inodeRetrieveCache1.
	if ntotal > 0 {
		st = OK
	}
	return ntotal, st
}

// inodeRetrieveCache1 is internal worker for InodeRetrieveCache which
// actually talks to kernel and retrieves chunks not larger than ms.opts.MaxWrite.
func (ms *Server) inodeRetrieveCache1(node uint64, offset int64, dest []byte) (n int, st Status) {
	if !ms.kernelSettings.SupportsNotify(NOTIFY_RETRIEVE_CACHE) {
		return 0, ENOSYS
	}

	req := request{
		inHeader: &InHeader{
			Opcode: _OP_NOTIFY_RETRIEVE_CACHE,
		},
		handler: operationHandlers[_OP_NOTIFY_RETRIEVE_CACHE],
		status:  NOTIFY_RETRIEVE_CACHE,
	}

	// retrieve up to 2GB not to overflow uint32 size in NotifyRetrieveOut.
	// see InodeNotifyStoreCache in similar place for why it is only 2GB, not 4GB.
	//
	// ( InodeRetrieveCache calls us with chunks not larger than
	//   ms.opts.MaxWrite, but MaxWrite is int, so let's be extra cautious )
	size := len(dest)
	if size > math.MaxInt32 {
		size = math.MaxInt32
	}
	dest = dest[:size]

	q := (*NotifyRetrieveOut)(req.outData())
	q.Nodeid = node
	q.Offset = uint64(offset) // not int64, as it is e.g. in NotifyInvalInodeOut
	q.Size = uint32(len(dest))

	reading := &retrieveCacheRequest{
		nodeid: q.Nodeid,
		offset: q.Offset,
		dest:   dest,
		ready:  make(chan struct{}),
	}

	ms.retrieveMu.Lock()
	q.NotifyUnique = ms.retrieveNext
	ms.retrieveNext++
	ms.retrieveTab[q.NotifyUnique] = reading
	ms.retrieveMu.Unlock()

	// Protect against concurrent close.
	ms.writeMu.Lock()
	result := ms.write(&req)
	ms.writeMu.Unlock()

	if ms.opts.Debug {
		log.Printf("Response: NOTIFY_RETRIEVE_CACHE: %v", result)
	}
	if result != OK {
		ms.retrieveMu.Lock()
		r := ms.retrieveTab[q.NotifyUnique]
		if r == reading {
			delete(ms.retrieveTab, q.NotifyUnique)
		} else if r == nil {
			// ok - might be dequeued by umount
		} else {
			// although very unlikely, it is possible that kernel sends
			// unexpected NotifyReply with our notifyUnique, then
			// retrieveNext wraps, makes full cycle, and another
			// retrieve request is made with the same notifyUnique.
			log.Printf("W: INODE_RETRIEVE_CACHE: request with notifyUnique=%d mutated", q.NotifyUnique)
		}
		ms.retrieveMu.Unlock()
		return 0, result
	}

	// NotifyRetrieveOut sent to the kernel successfully. Now the kernel
	// have to return data in a separate write-style NotifyReply request.
	// Wait for the result.
	<-reading.ready
	return reading.n, reading.st
}

// retrieveCacheRequest represents in-flight cache retrieve request.
type retrieveCacheRequest struct {
	nodeid uint64
	offset uint64
	dest   []byte

	// reply status
	n     int
	st    Status
	ready chan struct{}
}

// DeleteNotify notifies the kernel that an entry is removed from a
// directory.  In many cases, this is equivalent to EntryNotify,
// except when the directory is in use, eg. as working directory of
// some process. You should not hold any FUSE filesystem locks, as that
// can lead to deadlock.
func (ms *Server) DeleteNotify(parent uint64, child uint64, name string) Status {
	if ms.kernelSettings.Minor < 18 {
		return ms.EntryNotify(parent, name)
	}

	req := request{
		inHeader: &InHeader{
			Opcode: _OP_NOTIFY_DELETE,
		},
		handler: operationHandlers[_OP_NOTIFY_DELETE],
		status:  NOTIFY_DELETE,
	}

	entry := (*NotifyInvalDeleteOut)(req.outData())
	entry.Parent = parent
	entry.Child = child
	entry.NameLen = uint32(len(name))

	// Many versions of FUSE generate stacktraces if the
	// terminating null byte is missing.
	nameBytes := make([]byte, len(name)+1)
	copy(nameBytes, name)
	nameBytes[len(nameBytes)-1] = '\000'
	req.flatData = nameBytes

	// Protect against concurrent close.
	ms.writeMu.Lock()
	result := ms.write(&req)
	ms.writeMu.Unlock()

	if ms.opts.Debug {
		log.Printf("Response: DELETE_NOTIFY: %v", result)
	}
	return result
}

// EntryNotify should be used if the existence status of an entry
// within a directory changes. You should not hold any FUSE filesystem
// locks, as that can lead to deadlock.
func (ms *Server) EntryNotify(parent uint64, name string) Status {
	if !ms.kernelSettings.SupportsNotify(NOTIFY_INVAL_ENTRY) {
		return ENOSYS
	}
	req := request{
		inHeader: &InHeader{
			Opcode: _OP_NOTIFY_INVAL_ENTRY,
		},
		handler: operationHandlers[_OP_NOTIFY_INVAL_ENTRY],
		status:  NOTIFY_INVAL_ENTRY,
	}
	entry := (*NotifyInvalEntryOut)(req.outData())
	entry.Parent = parent
	entry.NameLen = uint32(len(name))

	// Many versions of FUSE generate stacktraces if the
	// terminating null byte is missing.
	nameBytes := make([]byte, len(name)+1)
	copy(nameBytes, name)
	nameBytes[len(nameBytes)-1] = '\000'
	req.flatData = nameBytes

	// Protect against concurrent close.
	ms.writeMu.Lock()
	result := ms.write(&req)
	ms.writeMu.Unlock()

	if ms.opts.Debug {
		log.Printf("Response: ENTRY_NOTIFY: %v", result)
	}
	return result
}

// SupportsVersion returns true if the kernel supports the given
// protocol version or newer.
func (in *InitIn) SupportsVersion(maj, min uint32) bool {
	return in.Major >= maj || (in.Major == maj && in.Minor >= min)
}

// SupportsNotify returns whether a certain notification type is
// supported. Pass any of the NOTIFY_* types as argument.
func (in *InitIn) SupportsNotify(notifyType int) bool {
	switch notifyType {
	case NOTIFY_INVAL_ENTRY:
		return in.SupportsVersion(7, 12)
	case NOTIFY_INVAL_INODE:
		return in.SupportsVersion(7, 12)
	case NOTIFY_STORE_CACHE, NOTIFY_RETRIEVE_CACHE:
		return in.SupportsVersion(7, 15)
	case NOTIFY_DELETE:
		return in.SupportsVersion(7, 18)
	}
	return false
}

var defaultBufferPool BufferPool

func init() {
	defaultBufferPool = NewBufferPool()
}

// WaitMount waits for the first request to be served. Use this to
// avoid racing between accessing the (empty or not yet mounted)
// mountpoint, and the OS trying to setup the user-space mount.
func (ms *Server) WaitMount() error {
	err := <-ms.ready
	if err != nil {
		return err
	}
	return pollHack(ms.mountPoint)
}
