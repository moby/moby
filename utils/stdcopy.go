package utils

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	StdWriterPrefixLen = 8
	StdWriterFdIndex   = 0
	StdWriterSizeIndex = 4
)

type StdType [StdWriterPrefixLen]byte

var (
	Stdin  StdType = StdType{0: 0}
	Stdout StdType = StdType{0: 1}
	Stderr StdType = StdType{0: 2}
)

type StdWriter struct {
	io.Writer
	prefix  StdType
	sizeBuf []byte
}

func (w *StdWriter) Write(buf []byte) (n int, err error) {
	if w == nil || w.Writer == nil {
		return 0, errors.New("Writer not instanciated")
	}
	binary.BigEndian.PutUint32(w.prefix[4:], uint32(len(buf)))
	buf = append(w.prefix[:], buf...)

	n, err = w.Writer.Write(buf)
	return n - StdWriterPrefixLen, err
}

// NewStdWriter instanciate a new Writer based on the given type `t`.
// the utils package contains the valid parametres for `t`:
func NewStdWriter(w io.Writer, t StdType) *StdWriter {
	if len(t) != StdWriterPrefixLen {
		return nil
	}

	return &StdWriter{
		Writer:  w,
		prefix:  t,
		sizeBuf: make([]byte, 4),
	}
}

var ErrInvalidStdHeader = errors.New("Unrecognized input header")

// StdCopy is a modified version of io.Copy.
//
// StdCopy copies from src to dstout or dsterr until either EOF is reached
// on src or an error occurs.  It returns the number of bytes
// copied and the first error encountered while copying, if any.
//
// A successful Copy returns err == nil, not err == EOF.
// Because Copy is defined to read from src until EOF, it does
// not treat an EOF from Read as an error to be reported.
//
// The source needs to be writter via StdWriter, dstout or dsterr is selected
// based on the prefix added by StdWriter
func StdCopy(dstout, dsterr io.Writer, src io.Reader) (written int64, err error) {
	var (
		buf       = make([]byte, 32*1024+StdWriterPrefixLen+1)
		bufLen    = len(buf)
		nr, nw    int
		er, ew    error
		out       io.Writer
		frameSize int
	)

	for {
		// Make sure we have at least a full header
		for nr < StdWriterPrefixLen {
			var nr2 int
			nr2, er = src.Read(buf[nr:])
			if er == io.EOF {
				return written, nil
			}
			if er != nil {
				return 0, er
			}
			nr += nr2
		}

		// Check the first byte to know where to write
		switch buf[StdWriterFdIndex] {
		case 0:
			fallthrough
		case 1:
			// Write on stdout
			out = dstout
		case 2:
			// Write on stderr
			out = dsterr
		default:
			Debugf("Error selecting output fd: (%d)", buf[StdWriterFdIndex])
			return 0, ErrInvalidStdHeader
		}

		// Retrieve the size of the frame
		frameSize = int(binary.BigEndian.Uint32(buf[StdWriterSizeIndex : StdWriterSizeIndex+4]))

		// Check if the buffer is big enough to read the frame.
		// Extend it if necessary.
		if frameSize+StdWriterPrefixLen > bufLen {
			Debugf("Extending buffer cap.")
			buf = append(buf, make([]byte, frameSize-len(buf)+1)...)
			bufLen = len(buf)
		}

		// While the amount of bytes read is less than the size of the frame + header, we keep reading
		for nr < frameSize+StdWriterPrefixLen {
			var nr2 int
			nr2, er = src.Read(buf[nr:])
			if er == io.EOF {
				return written, nil
			}
			if er != nil {
				Debugf("Error reading frame: %s", er)
				return 0, er
			}
			nr += nr2
		}

		// Write the retrieved frame (without header)
		nw, ew = out.Write(buf[StdWriterPrefixLen : frameSize+StdWriterPrefixLen])
		if nw > 0 {
			written += int64(nw)
		}
		if ew != nil {
			Debugf("Error writing frame: %s", ew)
			return 0, ew
		}
		// If the frame has not been fully written: error
		if nw != frameSize {
			Debugf("Error Short Write: (%d on %d)", nw, frameSize)
			return 0, io.ErrShortWrite
		}

		// Move the rest of the buffer to the beginning
		copy(buf, buf[frameSize+StdWriterPrefixLen:])
		// Move the index
		nr -= frameSize + StdWriterPrefixLen
	}
}
