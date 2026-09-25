package fs

import "os"

func getLinkInfo(_ os.FileInfo) (uint64, bool) {
	return 0, false
}
