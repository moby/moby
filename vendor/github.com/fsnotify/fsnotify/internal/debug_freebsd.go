package internal

import "golang.org/x/sys/unix"

var names = []struct {
	n string
	m uint32
}{
	{"NOTE_DELETE", unix.NOTE_DELETE},
	{"NOTE_WRITE", unix.NOTE_WRITE},
	{"NOTE_EXTEND", unix.NOTE_EXTEND},
	{"NOTE_ATTRIB", unix.NOTE_ATTRIB},
	{"NOTE_LINK", unix.NOTE_LINK},
	{"NOTE_RENAME", unix.NOTE_RENAME},
	{"NOTE_REVOKE", unix.NOTE_REVOKE},
	{"NOTE_OPEN", unix.NOTE_OPEN},
	{"NOTE_CLOSE", unix.NOTE_CLOSE},
	{"NOTE_CLOSE_WRITE", unix.NOTE_CLOSE_WRITE},
	{"NOTE_READ", unix.NOTE_READ},
}
