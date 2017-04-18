package cpio

import (
	"errors"
	"io"
	"strconv"
)

type Reader struct {
	r               io.Reader
	pos             int64
	remaining_bytes int
}

func NewReader(r io.Reader) *Reader {
	return &Reader{
		r: r,
	}
}

func disassemble(mode int64) (fmode int64, ftype int64) {
	fmode = mode & 0xFFF
	ftype = (mode >> 12) & 0xF
	return
}

func getPrefix(buf *[]byte, len int) (pre []byte) {
	pre, *buf = (*buf)[0:len], (*buf)[len:]
	return
}

func Btoi(s string, base int) (int, error) {
	i, e := strconv.ParseInt(s, base, 64)
	return int(i), e
}

var (
	ErrInvalidHeader = errors.New("Did not find valid magic number")
)

func parseHeader(buf []byte) (*Header, int64, error) {
	magic := string(getPrefix(&buf, 6))
	raw_inode := getPrefix(&buf, 8)
	raw_mode := getPrefix(&buf, 8)
	raw_uid := getPrefix(&buf, 8)
	raw_gid := getPrefix(&buf, 8)
	raw_nlinks := getPrefix(&buf, 8)
	raw_mtime := getPrefix(&buf, 8)
	raw_size := getPrefix(&buf, 8)
	raw_major := getPrefix(&buf, 8)
	raw_minor := getPrefix(&buf, 8)
	raw_devmajor := getPrefix(&buf, 8)
	raw_devminor := getPrefix(&buf, 8)
	raw_namelen := getPrefix(&buf, 8)
	raw_check := getPrefix(&buf, 8)

	_, _, _, _, _ = raw_inode, raw_nlinks, raw_major, raw_minor, raw_check

	if magic != "070701" {
		return nil, 0, ErrInvalidHeader
	}

	hdr := &Header{}

	mode, e := strconv.ParseInt(string(raw_mode), 16, 64)
	if e != nil {
		return nil, 0, e
	}

	hdr.Mode, hdr.Type = disassemble(mode)

	hdr.Uid, e = Btoi(string(raw_uid), 16)
	if e != nil {
		return nil, 0, e
	}

	hdr.Gid, e = Btoi(string(raw_gid), 16)
	if e != nil {
		return nil, 0, e
	}

	hdr.Mtime, e = strconv.ParseInt(string(raw_mtime), 16, 64)
	if e != nil {
		return nil, 0, e
	}

	hdr.Size, e = strconv.ParseInt(string(raw_size), 16, 64)
	if e != nil {
		return nil, 0, e
	}

	hdr.Devmajor, e = strconv.ParseInt(string(raw_devmajor), 16, 64)
	if e != nil {
		return nil, 0, e
	}

	hdr.Devminor, e = strconv.ParseInt(string(raw_devminor), 16, 64)
	if e != nil {
		return nil, 0, e
	}

	namelen, e := strconv.ParseInt(string(raw_namelen), 16, 64)
	if e != nil {
		return nil, 0, e
	}

	return hdr, namelen, nil
}

func (r *Reader) Next() (*Header, error) {
	e := r.skipRest()
	if e != nil {
		return nil, e
	}
	e = r.skipPadding(4)
	if e != nil {
		return nil, e
	}

	raw_hdr := make([]byte, 110)
	_, e = r.countedRead(raw_hdr)
	if e != nil {
		return nil, e
	}

	hdr, namelen, e := parseHeader(raw_hdr)
	if e != nil {
		return nil, e
	}

	bname := make([]byte, namelen)
	_, e = r.countedRead(bname)
	if e != nil {
		return nil, e
	}

	hdr.Name = string(bname[0 : namelen-1]) //Exclude terminating zero
	r.remaining_bytes = int(hdr.Size)
	return hdr, r.skipPadding(4)
}

func (r *Reader) skipRest() error {
	buf := make([]byte, 1)
	for ; r.remaining_bytes > 0; r.remaining_bytes-- {
		_, e := r.countedRead(buf)
		if e != nil {
			return e
		}
	}
	return nil
}

// Skips to the next position which is a multiple of mod.
func (r *Reader) skipPadding(mod int64) error {
	numBytesToRead := ((mod - (r.pos % mod)) % mod)
	buf := make([]byte, numBytesToRead)
	_, e := r.countedRead(buf)
	return e
}

func (r *Reader) Read(b []byte) (n int, e error) {
	if r.remaining_bytes == 0 {
		return 0, io.EOF
	}

	if len(b) > r.remaining_bytes {
		b = b[0:r.remaining_bytes]
	}
	n, e = r.countedRead(b)
	r.remaining_bytes -= n
	return
}

func (r *Reader) countedRead(b []byte) (n int, e error) {
	if len(b) == 0 {
		return
	}
	n, e = r.r.Read(b)
	r.pos += int64(n)
	return
}
