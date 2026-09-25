package internal

import "golang.org/x/sys/unix"

var names = []struct {
	n string
	m uint32
}{
	{"NOTE_ATTRIB", unix.NOTE_ATTRIB},
	{"NOTE_DELETE", unix.NOTE_DELETE},
	{"NOTE_EXTEND", unix.NOTE_EXTEND},
	{"NOTE_LINK", unix.NOTE_LINK},
	{"NOTE_RENAME", unix.NOTE_RENAME},
	{"NOTE_WRITE", unix.NOTE_WRITE},
}
