package fuse

// Go 1.9 introduces polling for file I/O. The implementation causes
// the runtime's epoll to take up the last GOMAXPROCS slot, and if
// that happens, we won't have any threads left to service FUSE's
// _OP_POLL request. Prevent this by forcing _OP_POLL to happen, so we
// can say ENOSYS and prevent further _OP_POLL requests.
const pollHackName = ".go-fuse-epoll-hack"
const pollHackInode = ^uint64(0)

func doPollHackLookup(ms *Server, req *request) {
	switch req.inHeader.Opcode {
	case _OP_CREATE:
		out := (*CreateOut)(req.outData())
		out.EntryOut = EntryOut{
			NodeId: pollHackInode,
			Attr: Attr{
				Ino:   pollHackInode,
				Mode:  S_IFREG | 0644,
				Nlink: 1,
			},
		}
		out.OpenOut = OpenOut{
			Fh: pollHackInode,
		}
		req.status = OK
	case _OP_LOOKUP:
		out := (*EntryOut)(req.outData())
		*out = EntryOut{}
		req.status = ENOENT
	default:
		req.status = EIO
	}
}
