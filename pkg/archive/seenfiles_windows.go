// +build windows

package archive

import "os"

func (sf SeenFiles) Add(fi os.FileInfo) {
	// length plus one. Not an inode, but still usable by systems that don't have inodes
	sf[uint64(len(sf))] = fi.Name()
}

func (sf SeenFiles) Include(fi os.FileInfo) string {
	n := fi.Name()
	for _, path := range sf {
		if n == path {
			return path
		}
	}
	return ""
}
