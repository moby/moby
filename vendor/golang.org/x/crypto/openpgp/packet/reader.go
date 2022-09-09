// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package packet

import (
	"golang.org/x/crypto/openpgp/errors"
	"io"
)

// Reader reads packets from an io.Reader and allows packets to be 'unread' so
// that they result from the next call to Next.
type Reader struct {
	q       []Packet
	readers []io.Reader
}

// New io.Readers are pushed when a compressed or encrypted packet is processed
// and recursively treated as a new source of packets. However, a carefully
// crafted packet can trigger an infinite recursive sequence of packets. See
// http://mumble.net/~campbell/misc/pgp-quine
// https://web.nvd.nist.gov/view/vuln/detail?vulnId=CVE-2013-4402
// This constant limits the number of recursive packets that may be pushed.
const maxReaders = 32

// Next returns the most recently unread Packet, or reads another packet from
// the top-most io.Reader. Unknown packet types are skipped.
func (r *Reader) Next() (p Packet, err error) {
	if len(r.q) > 0 {
		p = r.q[len(r.q)-1]
		r.q = r.q[:len(r.q)-1]
		return
	}

	for len(r.readers) > 0 {
		p, err = Read(r.readers[len(r.readers)-1])
		if err == nil {
			return
		}
		if err == io.EOF {
			r.readers = r.readers[:len(r.readers)-1]
			continue
		}
		if _, ok := err.(errors.UnknownPacketTypeError); !ok {
			return nil, err
		}
	}

	return nil, io.EOF
}

// Push causes the Reader to start reading from a new io.Reader. When an EOF
// error is seen from the new io.Reader, it is popped and the Reader continues
// to read from the next most recent io.Reader. Push returns a StructuralError
// if pushing the reader would exceed the maximum recursion level, otherwise it
// returns nil.
func (r *Reader) Push(reader io.Reader) (err error) {
	if len(r.readers) >= maxReaders {
		return errors.StructuralError("too many layers of packets")
	}
	r.readers = append(r.readers, reader)
	return nil
}

// Unread causes the given Packet to be returned from the next call to Next.
func (r *Reader) Unread(p Packet) {
	r.q = append(r.q, p)
}

func NewReader(r io.Reader) *Reader {
	return &Reader{
		q:       nil,
		readers: []io.Reader{r},
	}
}
