package fuse

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
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

	singleReader bool
	canSplice    bool
	loops        sync.WaitGroup
}

// SetDebug is deprecated. Use MountOptions.Debug instead.
func (ms *Server) SetDebug(dbg bool) {
	// This will typically trigger the race detector.
	ms.opts.Debug = dbg
}

// KernelSettings returns the Init message from the kernel, so
// filesystems can adapt to availability of features of the kernel
// driver.
func (ms *Server) KernelSettings() InitIn {
	ms.reqMu.Lock()
	s := ms.kernelSettings
	ms.reqMu.Unlock()

	return s
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
	opts = &o
	ms := &Server{
		fileSystem: fs,
		opts:       &o,
		// OSX has races when multiple routines read from the
		// FUSE device: on unmount, sometime some reads do not
		// error-out, meaning that unmount will hang.
		singleReader: runtime.GOOS == "darwin",
	}
	ms.reqPool.New = func() interface{} { return new(request) }
	ms.readPool.New = func() interface{} { return make([]byte, o.MaxWrite+PAGESIZE) }
	optStrs := opts.Options
	if opts.AllowOther {
		optStrs = append(optStrs, "allow_other")
	}

	name := opts.Name
	if name == "" {
		name = ms.fileSystem.String()
		l := len(name)
		if l > _MAX_NAME_LEN {
			l = _MAX_NAME_LEN
		}
		name = strings.Replace(name[:l], ",", ";", -1)
	}
	optStrs = append(optStrs, "subtype="+name)

	fsname := opts.FsName
	if len(fsname) > 0 {
		optStrs = append(optStrs, "fsname="+fsname)
	}

	mountPoint = filepath.Clean(mountPoint)
	if !filepath.IsAbs(mountPoint) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		mountPoint = filepath.Clean(filepath.Join(cwd, mountPoint))
	}
	fd, err := mount(mountPoint, strings.Join(optStrs, ","))
	if err != nil {
		return nil, err
	}

	ms.mountPoint = mountPoint
	ms.mountFd = fd

	ms.handleInit()

	return ms, nil
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
}

func (ms *Server) handleInit() {
	// The first request should be INIT; read it synchronously,
	// and don't spawn new readers.
	orig := ms.singleReader
	ms.singleReader = true
	req, errNo := ms.readRequest(false)
	ms.singleReader = orig

	if errNo != OK || req == nil {
		return
	}
	ms.handleRequest(req)

	// INIT is handled. Init the file system, but don't accept
	// incoming requests, so the file system can setup itself.
	ms.fileSystem.Init(ms)
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

func (ms *Server) handleRequest(req *request) {
	req.parse()
	if req.handler == nil {
		req.status = ENOSYS
	}

	if req.status.Ok() && ms.opts.Debug {
		log.Println(req.InputDebug())
	}

	if req.status.Ok() && req.handler.Func == nil {
		log.Printf("Unimplemented opcode %v", operationName(req.inHeader.Opcode))
		req.status = ENOSYS
	}

	if req.status.Ok() {
		req.handler.Func(ms, req)
	}

	errNo := ms.write(req)
	if errNo != 0 {
		log.Printf("writer: Write/Writev failed, err: %v. opcode: %v",
			errNo, operationName(req.inHeader.Opcode))
	}
	ms.returnRequest(req)
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
	// Forget does not wait for reply.
	if req.inHeader.Opcode == _OP_FORGET || req.inHeader.Opcode == _OP_BATCH_FORGET {
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
	entry := &NotifyInvalInodeOut{
		Ino:    node,
		Off:    off,
		Length: length,
	}
	req := request{
		inHeader: &InHeader{
			Opcode: _OP_NOTIFY_INODE,
		},
		handler: operationHandlers[_OP_NOTIFY_INODE],
		status:  NOTIFY_INVAL_INODE,
	}
	req.outData = unsafe.Pointer(entry)

	// Protect against concurrent close.
	ms.writeMu.Lock()
	result := ms.write(&req)
	ms.writeMu.Unlock()

	if ms.opts.Debug {
		log.Println("Response: INODE_NOTIFY", result)
	}
	return result
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
		status:  NOTIFY_INVAL_DELETE,
	}
	entry := &NotifyInvalDeleteOut{
		Parent:  parent,
		Child:   child,
		NameLen: uint32(len(name)),
	}

	// Many versions of FUSE generate stacktraces if the
	// terminating null byte is missing.
	nameBytes := make([]byte, len(name)+1)
	copy(nameBytes, name)
	nameBytes[len(nameBytes)-1] = '\000'
	req.outData = unsafe.Pointer(entry)
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
	req := request{
		inHeader: &InHeader{
			Opcode: _OP_NOTIFY_ENTRY,
		},
		handler: operationHandlers[_OP_NOTIFY_ENTRY],
		status:  NOTIFY_INVAL_ENTRY,
	}
	entry := &NotifyInvalEntryOut{
		Parent:  parent,
		NameLen: uint32(len(name)),
	}

	// Many versions of FUSE generate stacktraces if the
	// terminating null byte is missing.
	nameBytes := make([]byte, len(name)+1)
	copy(nameBytes, name)
	nameBytes[len(nameBytes)-1] = '\000'
	req.outData = unsafe.Pointer(entry)
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

var defaultBufferPool BufferPool

func init() {
	defaultBufferPool = NewBufferPool()
}

// WaitMount waits for the first request to be served. Use this to
// avoid racing between accessing the (empty) mountpoint, and the OS
// trying to setup the user-space mount.
func (ms *Server) WaitMount() {
	// TODO(hanwen): see if this can safely be removed.
}
