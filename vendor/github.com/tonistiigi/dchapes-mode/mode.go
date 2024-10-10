/*

Parts of this file are a heavily modified C to Go
translation of BSD's /usr/src/lib/libc/gen/setmode.c
that contains the following copyright notice:

 * Copyright (c) 1989, 1993, 1994
 *	The Regents of the University of California.  All rights reserved.
 *
 * This code is derived from software contributed to Berkeley by
 * Dave Borman at Cray Research, Inc.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 * 4. Neither the name of the University nor the names of its contributors
 *    may be used to endorse or promote products derived from this software
 *    without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE REGENTS AND CONTRIBUTORS ``AS IS'' AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED.  IN NO EVENT SHALL THE REGENTS OR CONTRIBUTORS BE LIABLE
 * FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
 * OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
 * LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
 * OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
 * SUCH DAMAGE.

*/

// Package mode provides a native Go implementation of BSD's setmode and getmode
// which can be used to modify the mode bits of an os.FileMode value based on
// a symbolic value as described by the Unix chmod command.
//
// For a full description of the mode string see chmod(1).
// Some examples include:
//
//	644		make a file readable by anyone and writable by the owner
//			only.
//
//	go-w		deny write permission to group and others.
//
//	=rw,+X		set the read and write permissions to the usual defaults,
//			but retain any execute permissions that are currently set.
//
//	+X		make a directory or file searchable/executable by everyone
//			if it is already searchable/executable by anyone.
//
//	755
//	u=rwx,go=rx
//	u=rwx,go=u-w	make a file readable/executable by everyone and writable by
//			the owner only.
//
//	go=		clear all mode bits for group and others.
//
//	go=u-w		set the group bits equal to the user bits, but clear the
//			group write bit.
//
// See Also:
//
//	setmode(3): https://www.freebsd.org/cgi/man.cgi?query=setmode&sektion=3
//	chmod(1):   https://www.freebsd.org/cgi/man.cgi?query=chmod&sektion=1
package mode

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Set is a set of changes to apply to an os.FileMode.
// Changes include setting or clearing specific bits, copying bits from one
// user class to another (e.g. "u=go" sets the user permissions to a copy of
// the group and other permsissions), etc.
type Set struct {
	cmds []bitcmd
}

type bitcmd struct {
	cmd  byte
	cmd2 byte
	bits modet
}

const (
	cmd2Clear byte = 1 << iota
	cmd2Set
	cmd2GBits
	cmd2OBits
	cmd2UBits
)

func (c bitcmd) String() string {
	c2 := ""
	if c.cmd2 != 0 {
		c2 = " cmd2:"
		if c.cmd2&cmd2Clear != 0 {
			c2 += " CLR"
		}
		if c.cmd2&cmd2Set != 0 {
			c2 += " SET"
		}
		if c.cmd2&cmd2UBits != 0 {
			c2 += " UBITS"
		}
		if c.cmd2&cmd2GBits != 0 {
			c2 += " GBITS"
		}
		if c.cmd2&cmd2OBits != 0 {
			c2 += " OBITS"
		}
	}
	return fmt.Sprintf("cmd: %q bits %#05o%s", c.cmd, c.bits, c2)
}

// The String method will likely only be useful when testing.
func (s Set) String() string {
	var buf strings.Builder
	buf.Grow(21*len(s.cmds) + 10)
	_, _ = buf.WriteString("set: {\n")
	for _, c := range s.cmds {
		_, _ = buf.WriteString(c.String())
		_ = buf.WriteByte('\n')
	}
	_, _ = buf.WriteString("}")
	return buf.String()
}

// ErrSyntax indicates an argument does not represent a valid mode.
var ErrSyntax = errors.New("invalid syntax")

// Apply changes the provided os.FileMode based on the given umask and
// absolute or symbolic mode value.
//
// Apply is a convience to calling ParseWithUmask followed by Apply.
// Since it needs to parse the mode value string on each call it
// should only be used when mode value string will not be reapplied.
func Apply(s string, perm os.FileMode, umask uint) (os.FileMode, error) {
	set, err := ParseWithUmask(s, umask)
	if err != nil {
		return 0, err
	}
	return set.Apply(perm), nil
}

// Parse takes an absolute (octal) or symbolic mode value,
// as described in chmod(1), as an argument and returns
// the set of bit operations representing the mode value
// that can be applied to specific os.FileMode values.
//
// Same as ParseWithUmask(s, 0).
func Parse(s string) (Set, error) {
	return ParseWithUmask(s, 0)
}

// TODO(dchapes): A Set.Parse method that reuses existing memory.

// TODO(dchapes): Only call syscall.Umask when abosolutely necessary and
// provide a Set method to query if set is umask dependant (and perhaps
// the umask that was in effect when parsed).

// ParseWithUmask is like Parse but uses the provided
// file creation mask instead of calling syscall.Umask.
func ParseWithUmask(s string, umask uint) (Set, error) {
	var m Set
	if s == "" {
		return m, ErrSyntax
	}

	// If an absolute number, get it and return;
	// disallow non-octal digits or illegal bits.
	if d := s[0]; '0' <= d && d <= '9' {
		v, err := strconv.ParseInt(s, 8, 16)
		if err != nil {
			return m, err
		}
		if v&^(standardBits|isTXT) != 0 {
			return m, ErrSyntax
		}
		// We know this takes exactly two bitcmds.
		m.cmds = make([]bitcmd, 0, 2)
		m.addcmd('=', standardBits|isTXT, modet(v), 0)
		return m, nil
	}

	// Get a copy of the mask for the permissions that are mask relative.
	// Flip the bits, we want what's not set.
	var mask modet = ^modet(umask)

	// Pre-allocate room for several commands.
	//m.cmds = make([]bitcmd, 0, 8)

	// Build list of bitcmd structs to set/clear/copy bits as described by
	// each clause of the symbolic mode.
	equalOpDone := false
	for {
		// First, find out which bits might be modified.
		var who modet
	whoLoop:
		for {
			if len(s) == 0 {
				return Set{}, ErrSyntax
			}
			switch s[0] {
			case 'a':
				who |= standardBits
			case 'u':
				who |= isUID | iRWXU
			case 'g':
				who |= isGID | iRWXG
			case 'o':
				who |= iRWXO
			default:
				break whoLoop
			}
			s = s[1:]
		}

		var op byte
	getop:
		op, s = s[0], s[1:]
		switch op {
		case '+', '-':
			// Nothing.
		case '=':
			equalOpDone = false
		default:
			return Set{}, ErrSyntax
		}

		who &^= isTXT
	permLoop:
		for perm, permX := modet(0), modet(0); ; s = s[1:] {
			var b byte
			if len(s) > 0 {
				b = s[0]
			}
			switch b {
			case 'r':
				perm |= iRUser | iRGroup | iROther
			case 's':
				// If only "other" bits ignore set-id.
				if who == 0 || who&^iRWXO != 0 {
					perm |= isUID | isGID
				}
			case 't':
				// If only "other bits ignore sticky.
				if who == 0 || who&^iRWXO != 0 {
					who |= isTXT
					perm |= isTXT
				}
			case 'w':
				perm |= iWUser | iWGroup | iWOther
			case 'X':
				if op == '+' {
					permX = iXUser | iXGroup | iXOther
				}
			case 'x':
				perm |= iXUser | iXGroup | iXOther
			case 'u', 'g', 'o':
				// Whenever we hit 'u', 'g', or 'o', we have
				// to flush out any partial mode that we have,
				// and then do the copying of the mode bits.
				if perm != 0 {
					m.addcmd(op, who, perm, mask)
					perm = 0
				}
				if op == '=' {
					equalOpDone = true
				}
				if permX != 0 {
					m.addcmd('X', who, permX, mask)
					permX = 0
				}
				m.addcmd(b, who, modet(op), mask)
			default:
				// Add any permissions that we haven't alread done.
				if perm != 0 || op == '=' && !equalOpDone {
					if op == '=' {
						equalOpDone = true
					}
					m.addcmd(op, who, perm, mask)
					//perm = 0
				}
				if permX != 0 {
					m.addcmd('X', who, permX, mask)
					//permX = 0
				}
				break permLoop
			}
		}

		if s == "" {
			break
		}
		if s[0] != ',' {
			goto getop
		}
		s = s[1:]
	}

	m.compress()
	return m, nil
}

// Apply returns the os.FileMode after applying the set of changes.
func (s Set) Apply(perm os.FileMode) os.FileMode {
	omode := fileModeToBits(perm)
	newmode := omode

	// When copying the user, group or other bits around, we "know"
	// where the bits are in the mode so that we can do shifts to
	// copy them around.  If we don't use shifts, it gets real
	// grundgy with lots of single bit checks and bit sets.
	common := func(c bitcmd, value modet) {
		if c.cmd2&cmd2Clear != 0 {
			var clrval modet
			if c.cmd2&cmd2Set != 0 {
				clrval = iRWXO
			} else {
				clrval = value
			}
			if c.cmd2&cmd2UBits != 0 {
				newmode &^= clrval << 6 & c.bits
			}
			if c.cmd2&cmd2GBits != 0 {
				newmode &^= clrval << 3 & c.bits
			}
			if c.cmd2&cmd2OBits != 0 {
				newmode &^= clrval & c.bits
			}
		}
		if c.cmd2&cmd2Set != 0 {
			if c.cmd2&cmd2UBits != 0 {
				newmode |= value << 6 & c.bits
			}
			if c.cmd2&cmd2GBits != 0 {
				newmode |= value << 3 & c.bits
			}
			if c.cmd2&cmd2OBits != 0 {
				newmode |= value & c.bits
			}
		}
	}

	for _, c := range s.cmds {
		switch c.cmd {
		case 'u':
			common(c, newmode&iRWXU>>6)
		case 'g':
			common(c, newmode&iRWXG>>3)
		case 'o':
			common(c, newmode&iRWXO)

		case '+':
			newmode |= c.bits
		case '-':
			newmode &^= c.bits

		case 'X':
			if omode&(iXUser|iXGroup|iXOther) != 0 || perm.IsDir() {
				newmode |= c.bits
			}
		}
	}

	return bitsToFileMode(perm, newmode)
}

// Chmod is a convience routine that applies the changes in
// Set to the named file. To avoid some race conditions,
// it opens the file and uses os.File.Stat and
// os.File.Chmod rather than os.Stat and os.Chmod if possible.
func (s *Set) Chmod(name string) (old, new os.FileMode, err error) {
	if f, err := os.Open(name); err == nil { // nolint: vetshadow
		defer f.Close() // nolint: errcheck
		return s.ChmodFile(f)
	}
	// Fallback to os.Stat and os.Chmod if we
	// don't have permission to open the file.
	fi, err := os.Stat(name)
	if err != nil {
		return 0, 0, err
	}
	old = fi.Mode()
	new = s.Apply(old)
	if new != old {
		err = os.Chmod(name, new)
	}
	return old, new, err

}

// ChmodFile is a convience routine that applies
// the changes in Set to the open file f.
func (s *Set) ChmodFile(f *os.File) (old, new os.FileMode, err error) {
	fi, err := f.Stat()
	if err != nil {
		return 0, 0, err
	}
	old = fi.Mode()
	new = s.Apply(old)
	if new != old {
		err = f.Chmod(new)
	}
	return old, new, err
}

func (s *Set) addcmd(op byte, who, oparg, mask modet) {
	c := bitcmd{}
	switch op {
	case '=':
		c.cmd = '-'
		if who != 0 {
			c.bits = who
		} else {
			c.bits = standardBits
		}

		s.cmds = append(s.cmds, c)
		//c = bitcmd{} // reset, not actually needed
		op = '+'
		fallthrough
	case '+', '-', 'X':
		c.cmd = op
		if who != 0 {
			c.bits = who & oparg
		} else {
			c.bits = mask & oparg
		}

	case 'u', 'g', 'o':
		c.cmd = op
		if who != 0 {
			if who&iRUser != 0 {
				c.cmd2 |= cmd2UBits
			}
			if who&iRGroup != 0 {
				c.cmd2 |= cmd2GBits
			}
			if who&iROther != 0 {
				c.cmd2 |= cmd2OBits
			}
			c.bits = ^modet(0)
		} else {
			c.cmd2 = cmd2UBits | cmd2GBits | cmd2OBits
			c.bits = mask
		}

		switch oparg {
		case '+':
			c.cmd2 |= cmd2Set
		case '-':
			c.cmd2 |= cmd2Clear
		case '=':
			c.cmd2 |= cmd2Set | cmd2Clear
		}
	default:
		panic("unreachable")
	}
	s.cmds = append(s.cmds, c)
}

// compress by compacting consecutive '+', '-' and 'X'
// commands into at most 3 commands, one of each. The 'u',
// 'g' and 'o' commands continue to be separate. They could
// probably be compacted, but it's not worth the effort.
func (s *Set) compress() {
	//log.Println("before:", *m)
	//log.Println("Start compress:")
	j := 0
	for i := 0; i < len(s.cmds); i++ {
		c := s.cmds[i]
		//log.Println(" read", i, c)
		if strings.IndexByte("+-X", c.cmd) < 0 {
			// Copy over any 'u', 'g', and 'o' commands.
			if i != j {
				s.cmds[j] = c
			}
			//log.Println(" wrote", j, "from", i)
			j++
			continue
		}
		var setbits, clrbits, Xbits modet
		for ; i < len(s.cmds); i++ {
			c = s.cmds[i]
			//log.Println(" scan", i, c)
			switch c.cmd {
			case '-':
				clrbits |= c.bits
				setbits &^= c.bits
				Xbits &^= c.bits
				continue
			case '+':
				setbits |= c.bits
				clrbits &^= c.bits
				Xbits &^= c.bits
				continue
			case 'X':
				Xbits |= c.bits &^ setbits
				continue
			default:
				i--
			}
			break
		}
		if clrbits != 0 {
			s.cmds[j].cmd = '-'
			s.cmds[j].cmd2 = 0
			s.cmds[j].bits = clrbits
			//log.Println(" wrote", j, "clrbits")
			j++
		}
		if setbits != 0 {
			s.cmds[j].cmd = '+'
			s.cmds[j].cmd2 = 0
			s.cmds[j].bits = setbits
			//log.Println(" wrote", j, "setbits")
			j++
		}
		if Xbits != 0 {
			s.cmds[j].cmd = 'X'
			s.cmds[j].cmd2 = 0
			s.cmds[j].bits = Xbits
			//log.Println(" wrote", j, "Xbits")
			j++
		}
	}
	/*
		if len(m.cmds) != j {
			log.Println("compressed", len(m.cmds), "down to", j)
		}
	*/
	s.cmds = s.cmds[:j]
	//log.Println("after:", *m)
}
