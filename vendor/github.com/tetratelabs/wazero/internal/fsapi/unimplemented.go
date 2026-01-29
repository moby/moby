package fsapi

import experimentalsys "github.com/tetratelabs/wazero/experimental/sys"

func Adapt(f experimentalsys.File) File {
	if f, ok := f.(File); ok {
		return f
	}
	return unimplementedFile{f}
}

type unimplementedFile struct{ experimentalsys.File }

// IsNonblock implements File.IsNonblock
func (unimplementedFile) IsNonblock() bool {
	return false
}

// SetNonblock implements File.SetNonblock
func (unimplementedFile) SetNonblock(bool) experimentalsys.Errno {
	return experimentalsys.ENOSYS
}

// Poll implements File.Poll
func (unimplementedFile) Poll(Pflag, int32) (ready bool, errno experimentalsys.Errno) {
	return false, experimentalsys.ENOSYS
}
