package openfile

// Windows has neither O_NONBLOCK nor O_NOFOLLOW, and no fifos or device nodes
// in the filesystem namespace for them to guard against.
const (
	nonBlock = 0
	noFollow = 0
)
