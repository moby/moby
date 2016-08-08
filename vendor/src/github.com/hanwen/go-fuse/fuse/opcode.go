package fuse

import (
	"bytes"
	"log"
	"reflect"
	"runtime"
	"unsafe"
)

const (
	_OP_LOOKUP       = int32(1)
	_OP_FORGET       = int32(2)
	_OP_GETATTR      = int32(3)
	_OP_SETATTR      = int32(4)
	_OP_READLINK     = int32(5)
	_OP_SYMLINK      = int32(6)
	_OP_MKNOD        = int32(8)
	_OP_MKDIR        = int32(9)
	_OP_UNLINK       = int32(10)
	_OP_RMDIR        = int32(11)
	_OP_RENAME       = int32(12)
	_OP_LINK         = int32(13)
	_OP_OPEN         = int32(14)
	_OP_READ         = int32(15)
	_OP_WRITE        = int32(16)
	_OP_STATFS       = int32(17)
	_OP_RELEASE      = int32(18)
	_OP_FSYNC        = int32(20)
	_OP_SETXATTR     = int32(21)
	_OP_GETXATTR     = int32(22)
	_OP_LISTXATTR    = int32(23)
	_OP_REMOVEXATTR  = int32(24)
	_OP_FLUSH        = int32(25)
	_OP_INIT         = int32(26)
	_OP_OPENDIR      = int32(27)
	_OP_READDIR      = int32(28)
	_OP_RELEASEDIR   = int32(29)
	_OP_FSYNCDIR     = int32(30)
	_OP_GETLK        = int32(31)
	_OP_SETLK        = int32(32)
	_OP_SETLKW       = int32(33)
	_OP_ACCESS       = int32(34)
	_OP_CREATE       = int32(35)
	_OP_INTERRUPT    = int32(36)
	_OP_BMAP         = int32(37)
	_OP_DESTROY      = int32(38)
	_OP_IOCTL        = int32(39)
	_OP_POLL         = int32(40)
	_OP_NOTIFY_REPLY = int32(41)
	_OP_BATCH_FORGET = int32(42)
	_OP_FALLOCATE    = int32(43) // protocol version 19.
	_OP_READDIRPLUS  = int32(44) // protocol version 21.
	_OP_FUSE_RENAME2 = int32(45) // protocol version 23.

	// The following entries don't have to be compatible across Go-FUSE versions.
	_OP_NOTIFY_ENTRY  = int32(100)
	_OP_NOTIFY_INODE  = int32(101)
	_OP_NOTIFY_DELETE = int32(102) // protocol version 18

	_OPCODE_COUNT = int32(103)
)

////////////////////////////////////////////////////////////////

func doInit(server *Server, req *request) {
	input := (*InitIn)(req.inData)
	if input.Major != _FUSE_KERNEL_VERSION {
		log.Printf("Major versions does not match. Given %d, want %d\n", input.Major, _FUSE_KERNEL_VERSION)
		req.status = EIO
		return
	}
	if input.Minor < _MINIMUM_MINOR_VERSION {
		log.Printf("Minor version is less than we support. Given %d, want at least %d\n", input.Minor, _MINIMUM_MINOR_VERSION)
		req.status = EIO
		return
	}

	server.reqMu.Lock()
	server.kernelSettings = *input
	server.kernelSettings.Flags = input.Flags & (CAP_ASYNC_READ | CAP_BIG_WRITES | CAP_FILE_OPS |
		CAP_AUTO_INVAL_DATA | CAP_READDIRPLUS | CAP_NO_OPEN_SUPPORT)

	if input.Minor >= 13 {
		server.setSplice()
	}
	server.reqMu.Unlock()

	out := &InitOut{
		Major:               _FUSE_KERNEL_VERSION,
		Minor:               _OUR_MINOR_VERSION,
		MaxReadAhead:        input.MaxReadAhead,
		Flags:               server.kernelSettings.Flags,
		MaxWrite:            uint32(server.opts.MaxWrite),
		CongestionThreshold: uint16(server.opts.MaxBackground * 3 / 4),
		MaxBackground:       uint16(server.opts.MaxBackground),
	}
	if out.Minor > input.Minor {
		out.Minor = input.Minor
	}

	if out.Minor <= 22 {
		tweaked := *req.handler

		// v8-v22 don't have TimeGran and further fields.
		tweaked.OutputSize = 24
		req.handler = &tweaked
	}

	req.outData = unsafe.Pointer(out)
	req.status = OK
}

func doOpen(server *Server, req *request) {
	out := (*OpenOut)(req.outData)
	status := server.fileSystem.Open((*OpenIn)(req.inData), out)
	req.status = status
	if status != OK {
		return
	}
}

func doCreate(server *Server, req *request) {
	out := (*CreateOut)(req.outData)
	status := server.fileSystem.Create((*CreateIn)(req.inData), req.filenames[0], out)
	req.status = status
}

func doReadDir(server *Server, req *request) {
	in := (*ReadIn)(req.inData)
	buf := server.allocOut(req, in.Size)
	out := NewDirEntryList(buf, uint64(in.Offset))

	code := server.fileSystem.ReadDir(in, out)
	req.flatData = out.bytes()
	req.status = code
}

func doReadDirPlus(server *Server, req *request) {
	in := (*ReadIn)(req.inData)
	buf := server.allocOut(req, in.Size)
	out := NewDirEntryList(buf, uint64(in.Offset))

	code := server.fileSystem.ReadDirPlus(in, out)
	req.flatData = out.bytes()
	req.status = code
}

func doOpenDir(server *Server, req *request) {
	out := (*OpenOut)(req.outData)
	status := server.fileSystem.OpenDir((*OpenIn)(req.inData), out)
	req.status = status
}

func doSetattr(server *Server, req *request) {
	out := (*AttrOut)(req.outData)
	req.status = server.fileSystem.SetAttr((*SetAttrIn)(req.inData), out)
}

func doWrite(server *Server, req *request) {
	n, status := server.fileSystem.Write((*WriteIn)(req.inData), req.arg)
	o := (*WriteOut)(req.outData)
	o.Size = n
	req.status = status
}

const _SECURITY_CAPABILITY = "security.capability"
const _SECURITY_ACL = "system.posix_acl_access"
const _SECURITY_ACL_DEFAULT = "system.posix_acl_default"

func doGetXAttr(server *Server, req *request) {
	if server.opts.DisableXAttrs {
		req.status = ENOSYS
		return
	}

	if server.opts.IgnoreSecurityLabels && req.inHeader.Opcode == _OP_GETXATTR {
		fn := req.filenames[0]
		if fn == _SECURITY_CAPABILITY || fn == _SECURITY_ACL_DEFAULT ||
			fn == _SECURITY_ACL {
			req.status = ENODATA
			return
		}
	}

	input := (*GetXAttrIn)(req.inData)

	if input.Size == 0 {
		out := (*GetXAttrOut)(req.outData)
		switch req.inHeader.Opcode {
		case _OP_GETXATTR:
			// TODO(hanwen): double check this. For getxattr, input.Size
			// field refers to the size of the attribute, so it usually
			// is not 0.
			sz, code := server.fileSystem.GetXAttrSize(req.inHeader, req.filenames[0])
			if code.Ok() {
				out.Size = uint32(sz)
			}
			req.status = code
			return
		case _OP_LISTXATTR:
			data, code := server.fileSystem.ListXAttr(req.inHeader)
			if code.Ok() {
				out.Size = uint32(len(data))
			}
			req.status = code
			return
		}
	}

	req.outData = nil
	var data []byte
	switch req.inHeader.Opcode {
	case _OP_GETXATTR:
		data, req.status = server.fileSystem.GetXAttrData(req.inHeader, req.filenames[0])
	case _OP_LISTXATTR:
		data, req.status = server.fileSystem.ListXAttr(req.inHeader)
	default:
		log.Panicf("xattr opcode %v", req.inHeader.Opcode)
		req.status = ENOSYS
	}

	if len(data) > int(input.Size) {
		req.status = ERANGE
	}

	if !req.status.Ok() {
		return
	}

	req.flatData = data
}

func doGetAttr(server *Server, req *request) {
	out := (*AttrOut)(req.outData)
	s := server.fileSystem.GetAttr((*GetAttrIn)(req.inData), out)
	req.status = s
}

// doForget - forget one NodeId
func doForget(server *Server, req *request) {
	if !server.opts.RememberInodes {
		server.fileSystem.Forget(req.inHeader.NodeId, (*ForgetIn)(req.inData).Nlookup)
	}
}

// doBatchForget - forget a list of NodeIds
func doBatchForget(server *Server, req *request) {
	in := (*_BatchForgetIn)(req.inData)
	wantBytes := uintptr(in.Count) * unsafe.Sizeof(_ForgetOne{})
	if uintptr(len(req.arg)) < wantBytes {
		// We have no return value to complain, so log an error.
		log.Printf("Too few bytes for batch forget. Got %d bytes, want %d (%d entries)",
			len(req.arg), wantBytes, in.Count)
	}

	h := &reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&req.arg[0])),
		Len:  int(in.Count),
		Cap:  int(in.Count),
	}

	forgets := *(*[]_ForgetOne)(unsafe.Pointer(h))
	for i, f := range forgets {
		if server.opts.Debug {
			log.Printf("doBatchForget: forgetting %d of %d: NodeId: %d, Nlookup: %d", i+1, len(forgets), f.NodeId, f.Nlookup)
		}
		server.fileSystem.Forget(f.NodeId, f.Nlookup)
	}
}

func doReadlink(server *Server, req *request) {
	req.flatData, req.status = server.fileSystem.Readlink(req.inHeader)
}

func doLookup(server *Server, req *request) {
	out := (*EntryOut)(req.outData)
	s := server.fileSystem.Lookup(req.inHeader, req.filenames[0], out)
	req.status = s
	req.outData = unsafe.Pointer(out)
}

func doMknod(server *Server, req *request) {
	out := (*EntryOut)(req.outData)

	req.status = server.fileSystem.Mknod((*MknodIn)(req.inData), req.filenames[0], out)
}

func doMkdir(server *Server, req *request) {
	out := (*EntryOut)(req.outData)
	req.status = server.fileSystem.Mkdir((*MkdirIn)(req.inData), req.filenames[0], out)
}

func doUnlink(server *Server, req *request) {
	req.status = server.fileSystem.Unlink(req.inHeader, req.filenames[0])
}

func doRmdir(server *Server, req *request) {
	req.status = server.fileSystem.Rmdir(req.inHeader, req.filenames[0])
}

func doLink(server *Server, req *request) {
	out := (*EntryOut)(req.outData)
	req.status = server.fileSystem.Link((*LinkIn)(req.inData), req.filenames[0], out)
}

func doRead(server *Server, req *request) {
	in := (*ReadIn)(req.inData)
	buf := server.allocOut(req, in.Size)

	req.readResult, req.status = server.fileSystem.Read(in, buf)
	if fd, ok := req.readResult.(*readResultFd); ok {
		req.fdData = fd
		req.flatData = nil
	} else if req.readResult != nil && req.status.Ok() {
		req.flatData, req.status = req.readResult.Bytes(buf)
	}
}

func doFlush(server *Server, req *request) {
	req.status = server.fileSystem.Flush((*FlushIn)(req.inData))
}

func doRelease(server *Server, req *request) {
	server.fileSystem.Release((*ReleaseIn)(req.inData))
}

func doFsync(server *Server, req *request) {
	req.status = server.fileSystem.Fsync((*FsyncIn)(req.inData))
}

func doReleaseDir(server *Server, req *request) {
	server.fileSystem.ReleaseDir((*ReleaseIn)(req.inData))
}

func doFsyncDir(server *Server, req *request) {
	req.status = server.fileSystem.FsyncDir((*FsyncIn)(req.inData))
}

func doSetXAttr(server *Server, req *request) {
	splits := bytes.SplitN(req.arg, []byte{0}, 2)
	req.status = server.fileSystem.SetXAttr((*SetXAttrIn)(req.inData), string(splits[0]), splits[1])
}

func doRemoveXAttr(server *Server, req *request) {
	req.status = server.fileSystem.RemoveXAttr(req.inHeader, req.filenames[0])
}

func doAccess(server *Server, req *request) {
	req.status = server.fileSystem.Access((*AccessIn)(req.inData))
}

func doSymlink(server *Server, req *request) {
	out := (*EntryOut)(req.outData)
	req.status = server.fileSystem.Symlink(req.inHeader, req.filenames[1], req.filenames[0], out)
}

func doRename(server *Server, req *request) {
	req.status = server.fileSystem.Rename((*RenameIn)(req.inData), req.filenames[0], req.filenames[1])
}

func doStatFs(server *Server, req *request) {
	out := (*StatfsOut)(req.outData)
	req.status = server.fileSystem.StatFs(req.inHeader, out)
	if req.status == ENOSYS && runtime.GOOS == "darwin" {
		// OSX FUSE requires Statfs to be implemented for the
		// mount to succeed.
		*out = StatfsOut{}
		req.status = OK
	}
}

func doIoctl(server *Server, req *request) {
	req.status = ENOSYS
}

func doDestroy(server *Server, req *request) {
	req.status = OK
}

func doFallocate(server *Server, req *request) {
	req.status = server.fileSystem.Fallocate((*FallocateIn)(req.inData))
}

////////////////////////////////////////////////////////////////

type operationFunc func(*Server, *request)
type castPointerFunc func(unsafe.Pointer) interface{}

type operationHandler struct {
	Name        string
	Func        operationFunc
	InputSize   uintptr
	OutputSize  uintptr
	DecodeIn    castPointerFunc
	DecodeOut   castPointerFunc
	FileNames   int
	FileNameOut bool
}

var operationHandlers []*operationHandler

func operationName(op int32) string {
	h := getHandler(op)
	if h == nil {
		return "unknown"
	}
	return h.Name
}

func getHandler(o int32) *operationHandler {
	if o >= _OPCODE_COUNT {
		return nil
	}
	return operationHandlers[o]
}

func init() {
	operationHandlers = make([]*operationHandler, _OPCODE_COUNT)
	for i := range operationHandlers {
		operationHandlers[i] = &operationHandler{Name: "UNKNOWN"}
	}

	fileOps := []int32{_OP_READLINK, _OP_NOTIFY_ENTRY, _OP_NOTIFY_DELETE}
	for _, op := range fileOps {
		operationHandlers[op].FileNameOut = true
	}

	for op, sz := range map[int32]uintptr{
		_OP_FORGET:       unsafe.Sizeof(ForgetIn{}),
		_OP_BATCH_FORGET: unsafe.Sizeof(_BatchForgetIn{}),
		_OP_GETATTR:      unsafe.Sizeof(GetAttrIn{}),
		_OP_SETATTR:      unsafe.Sizeof(SetAttrIn{}),
		_OP_MKNOD:        unsafe.Sizeof(MknodIn{}),
		_OP_MKDIR:        unsafe.Sizeof(MkdirIn{}),
		_OP_RENAME:       unsafe.Sizeof(RenameIn{}),
		_OP_LINK:         unsafe.Sizeof(LinkIn{}),
		_OP_OPEN:         unsafe.Sizeof(OpenIn{}),
		_OP_READ:         unsafe.Sizeof(ReadIn{}),
		_OP_WRITE:        unsafe.Sizeof(WriteIn{}),
		_OP_RELEASE:      unsafe.Sizeof(ReleaseIn{}),
		_OP_FSYNC:        unsafe.Sizeof(FsyncIn{}),
		_OP_SETXATTR:     unsafe.Sizeof(SetXAttrIn{}),
		_OP_GETXATTR:     unsafe.Sizeof(GetXAttrIn{}),
		_OP_LISTXATTR:    unsafe.Sizeof(GetXAttrIn{}),
		_OP_FLUSH:        unsafe.Sizeof(FlushIn{}),
		_OP_INIT:         unsafe.Sizeof(InitIn{}),
		_OP_OPENDIR:      unsafe.Sizeof(OpenIn{}),
		_OP_READDIR:      unsafe.Sizeof(ReadIn{}),
		_OP_RELEASEDIR:   unsafe.Sizeof(ReleaseIn{}),
		_OP_FSYNCDIR:     unsafe.Sizeof(FsyncIn{}),
		_OP_ACCESS:       unsafe.Sizeof(AccessIn{}),
		_OP_CREATE:       unsafe.Sizeof(CreateIn{}),
		_OP_INTERRUPT:    unsafe.Sizeof(InterruptIn{}),
		_OP_BMAP:         unsafe.Sizeof(_BmapIn{}),
		_OP_IOCTL:        unsafe.Sizeof(_IoctlIn{}),
		_OP_POLL:         unsafe.Sizeof(_PollIn{}),
		_OP_FALLOCATE:    unsafe.Sizeof(FallocateIn{}),
		_OP_READDIRPLUS:  unsafe.Sizeof(ReadIn{}),
	} {
		operationHandlers[op].InputSize = sz
	}

	for op, sz := range map[int32]uintptr{
		_OP_LOOKUP:        unsafe.Sizeof(EntryOut{}),
		_OP_GETATTR:       unsafe.Sizeof(AttrOut{}),
		_OP_SETATTR:       unsafe.Sizeof(AttrOut{}),
		_OP_SYMLINK:       unsafe.Sizeof(EntryOut{}),
		_OP_MKNOD:         unsafe.Sizeof(EntryOut{}),
		_OP_MKDIR:         unsafe.Sizeof(EntryOut{}),
		_OP_LINK:          unsafe.Sizeof(EntryOut{}),
		_OP_OPEN:          unsafe.Sizeof(OpenOut{}),
		_OP_WRITE:         unsafe.Sizeof(WriteOut{}),
		_OP_STATFS:        unsafe.Sizeof(StatfsOut{}),
		_OP_GETXATTR:      unsafe.Sizeof(GetXAttrOut{}),
		_OP_LISTXATTR:     unsafe.Sizeof(GetXAttrOut{}),
		_OP_INIT:          unsafe.Sizeof(InitOut{}),
		_OP_OPENDIR:       unsafe.Sizeof(OpenOut{}),
		_OP_CREATE:        unsafe.Sizeof(CreateOut{}),
		_OP_BMAP:          unsafe.Sizeof(_BmapOut{}),
		_OP_IOCTL:         unsafe.Sizeof(_IoctlOut{}),
		_OP_POLL:          unsafe.Sizeof(_PollOut{}),
		_OP_NOTIFY_ENTRY:  unsafe.Sizeof(NotifyInvalEntryOut{}),
		_OP_NOTIFY_INODE:  unsafe.Sizeof(NotifyInvalInodeOut{}),
		_OP_NOTIFY_DELETE: unsafe.Sizeof(NotifyInvalDeleteOut{}),
	} {
		operationHandlers[op].OutputSize = sz
	}

	for op, v := range map[int32]string{
		_OP_LOOKUP:        "LOOKUP",
		_OP_FORGET:        "FORGET",
		_OP_BATCH_FORGET:  "BATCH_FORGET",
		_OP_GETATTR:       "GETATTR",
		_OP_SETATTR:       "SETATTR",
		_OP_READLINK:      "READLINK",
		_OP_SYMLINK:       "SYMLINK",
		_OP_MKNOD:         "MKNOD",
		_OP_MKDIR:         "MKDIR",
		_OP_UNLINK:        "UNLINK",
		_OP_RMDIR:         "RMDIR",
		_OP_RENAME:        "RENAME",
		_OP_LINK:          "LINK",
		_OP_OPEN:          "OPEN",
		_OP_READ:          "READ",
		_OP_WRITE:         "WRITE",
		_OP_STATFS:        "STATFS",
		_OP_RELEASE:       "RELEASE",
		_OP_FSYNC:         "FSYNC",
		_OP_SETXATTR:      "SETXATTR",
		_OP_GETXATTR:      "GETXATTR",
		_OP_LISTXATTR:     "LISTXATTR",
		_OP_REMOVEXATTR:   "REMOVEXATTR",
		_OP_FLUSH:         "FLUSH",
		_OP_INIT:          "INIT",
		_OP_OPENDIR:       "OPENDIR",
		_OP_READDIR:       "READDIR",
		_OP_RELEASEDIR:    "RELEASEDIR",
		_OP_FSYNCDIR:      "FSYNCDIR",
		_OP_GETLK:         "GETLK",
		_OP_SETLK:         "SETLK",
		_OP_SETLKW:        "SETLKW",
		_OP_ACCESS:        "ACCESS",
		_OP_CREATE:        "CREATE",
		_OP_INTERRUPT:     "INTERRUPT",
		_OP_BMAP:          "BMAP",
		_OP_DESTROY:       "DESTROY",
		_OP_IOCTL:         "IOCTL",
		_OP_POLL:          "POLL",
		_OP_NOTIFY_ENTRY:  "NOTIFY_ENTRY",
		_OP_NOTIFY_INODE:  "NOTIFY_INODE",
		_OP_NOTIFY_DELETE: "NOTIFY_DELETE",
		_OP_FALLOCATE:     "FALLOCATE",
		_OP_READDIRPLUS:   "READDIRPLUS",
	} {
		operationHandlers[op].Name = v
	}

	for op, v := range map[int32]operationFunc{
		_OP_OPEN:         doOpen,
		_OP_READDIR:      doReadDir,
		_OP_WRITE:        doWrite,
		_OP_OPENDIR:      doOpenDir,
		_OP_CREATE:       doCreate,
		_OP_SETATTR:      doSetattr,
		_OP_GETXATTR:     doGetXAttr,
		_OP_LISTXATTR:    doGetXAttr,
		_OP_GETATTR:      doGetAttr,
		_OP_FORGET:       doForget,
		_OP_BATCH_FORGET: doBatchForget,
		_OP_READLINK:     doReadlink,
		_OP_INIT:         doInit,
		_OP_LOOKUP:       doLookup,
		_OP_MKNOD:        doMknod,
		_OP_MKDIR:        doMkdir,
		_OP_UNLINK:       doUnlink,
		_OP_RMDIR:        doRmdir,
		_OP_LINK:         doLink,
		_OP_READ:         doRead,
		_OP_FLUSH:        doFlush,
		_OP_RELEASE:      doRelease,
		_OP_FSYNC:        doFsync,
		_OP_RELEASEDIR:   doReleaseDir,
		_OP_FSYNCDIR:     doFsyncDir,
		_OP_SETXATTR:     doSetXAttr,
		_OP_REMOVEXATTR:  doRemoveXAttr,
		_OP_ACCESS:       doAccess,
		_OP_SYMLINK:      doSymlink,
		_OP_RENAME:       doRename,
		_OP_STATFS:       doStatFs,
		_OP_IOCTL:        doIoctl,
		_OP_DESTROY:      doDestroy,
		_OP_FALLOCATE:    doFallocate,
		_OP_READDIRPLUS:  doReadDirPlus,
	} {
		operationHandlers[op].Func = v
	}

	// Outputs.
	for op, f := range map[int32]castPointerFunc{
		_OP_LOOKUP:        func(ptr unsafe.Pointer) interface{} { return (*EntryOut)(ptr) },
		_OP_OPEN:          func(ptr unsafe.Pointer) interface{} { return (*OpenOut)(ptr) },
		_OP_OPENDIR:       func(ptr unsafe.Pointer) interface{} { return (*OpenOut)(ptr) },
		_OP_GETATTR:       func(ptr unsafe.Pointer) interface{} { return (*AttrOut)(ptr) },
		_OP_CREATE:        func(ptr unsafe.Pointer) interface{} { return (*CreateOut)(ptr) },
		_OP_LINK:          func(ptr unsafe.Pointer) interface{} { return (*EntryOut)(ptr) },
		_OP_SETATTR:       func(ptr unsafe.Pointer) interface{} { return (*AttrOut)(ptr) },
		_OP_INIT:          func(ptr unsafe.Pointer) interface{} { return (*InitOut)(ptr) },
		_OP_MKDIR:         func(ptr unsafe.Pointer) interface{} { return (*EntryOut)(ptr) },
		_OP_NOTIFY_ENTRY:  func(ptr unsafe.Pointer) interface{} { return (*NotifyInvalEntryOut)(ptr) },
		_OP_NOTIFY_INODE:  func(ptr unsafe.Pointer) interface{} { return (*NotifyInvalInodeOut)(ptr) },
		_OP_NOTIFY_DELETE: func(ptr unsafe.Pointer) interface{} { return (*NotifyInvalDeleteOut)(ptr) },
		_OP_STATFS:        func(ptr unsafe.Pointer) interface{} { return (*StatfsOut)(ptr) },
		_OP_SYMLINK:       func(ptr unsafe.Pointer) interface{} { return (*EntryOut)(ptr) },
	} {
		operationHandlers[op].DecodeOut = f
	}

	// Inputs.
	for op, f := range map[int32]castPointerFunc{
		_OP_FLUSH:        func(ptr unsafe.Pointer) interface{} { return (*FlushIn)(ptr) },
		_OP_GETATTR:      func(ptr unsafe.Pointer) interface{} { return (*GetAttrIn)(ptr) },
		_OP_GETXATTR:     func(ptr unsafe.Pointer) interface{} { return (*GetXAttrIn)(ptr) },
		_OP_LISTXATTR:    func(ptr unsafe.Pointer) interface{} { return (*GetXAttrIn)(ptr) },
		_OP_SETATTR:      func(ptr unsafe.Pointer) interface{} { return (*SetAttrIn)(ptr) },
		_OP_INIT:         func(ptr unsafe.Pointer) interface{} { return (*InitIn)(ptr) },
		_OP_IOCTL:        func(ptr unsafe.Pointer) interface{} { return (*_IoctlIn)(ptr) },
		_OP_OPEN:         func(ptr unsafe.Pointer) interface{} { return (*OpenIn)(ptr) },
		_OP_MKNOD:        func(ptr unsafe.Pointer) interface{} { return (*MknodIn)(ptr) },
		_OP_CREATE:       func(ptr unsafe.Pointer) interface{} { return (*CreateIn)(ptr) },
		_OP_READ:         func(ptr unsafe.Pointer) interface{} { return (*ReadIn)(ptr) },
		_OP_READDIR:      func(ptr unsafe.Pointer) interface{} { return (*ReadIn)(ptr) },
		_OP_ACCESS:       func(ptr unsafe.Pointer) interface{} { return (*AccessIn)(ptr) },
		_OP_FORGET:       func(ptr unsafe.Pointer) interface{} { return (*ForgetIn)(ptr) },
		_OP_BATCH_FORGET: func(ptr unsafe.Pointer) interface{} { return (*_BatchForgetIn)(ptr) },
		_OP_LINK:         func(ptr unsafe.Pointer) interface{} { return (*LinkIn)(ptr) },
		_OP_MKDIR:        func(ptr unsafe.Pointer) interface{} { return (*MkdirIn)(ptr) },
		_OP_RELEASE:      func(ptr unsafe.Pointer) interface{} { return (*ReleaseIn)(ptr) },
		_OP_RELEASEDIR:   func(ptr unsafe.Pointer) interface{} { return (*ReleaseIn)(ptr) },
		_OP_FALLOCATE:    func(ptr unsafe.Pointer) interface{} { return (*FallocateIn)(ptr) },
		_OP_READDIRPLUS:  func(ptr unsafe.Pointer) interface{} { return (*ReadIn)(ptr) },
		_OP_RENAME:       func(ptr unsafe.Pointer) interface{} { return (*RenameIn)(ptr) },
	} {
		operationHandlers[op].DecodeIn = f
	}

	// File name args.
	for op, count := range map[int32]int{
		_OP_CREATE:      1,
		_OP_GETXATTR:    1,
		_OP_LINK:        1,
		_OP_LOOKUP:      1,
		_OP_MKDIR:       1,
		_OP_MKNOD:       1,
		_OP_REMOVEXATTR: 1,
		_OP_RENAME:      2,
		_OP_RMDIR:       1,
		_OP_SYMLINK:     2,
		_OP_UNLINK:      1,
	} {
		operationHandlers[op].FileNames = count
	}

	var r request
	sizeOfOutHeader := unsafe.Sizeof(OutHeader{})
	for code, h := range operationHandlers {
		if h.OutputSize+sizeOfOutHeader > unsafe.Sizeof(r.outBuf) {
			log.Panicf("request output buffer too small: code %v, sz %d + %d %v", code, h.OutputSize, sizeOfOutHeader, h)
		}
	}
}
