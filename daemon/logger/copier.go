package logger

import (
	"bufio"
	"bytes"
	"io"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

// maxScanBufferSize is the maximum size allowed
// for the log buffer to copy its data.
const maxScanBufferSize = bufio.MaxScanTokenSize / 2

type bufferedCopier interface {
	Scan() bool
	Bytes() ([]byte, error)
}

// Copier can copy logs from specified sources to Logger and attach
// ContainerID and Timestamp.
// Writes are concurrent, so you need implement some sync in your logger
type Copier struct {
	// cid is the container id for which we are copying logs
	cid string
	// srcs is map of name -> reader pairs, for example "stdout", "stderr"
	srcs       map[string]io.Reader
	dst        Logger
	copyJobs   sync.WaitGroup
	closed     chan struct{}
	copierFunc func(io.Reader) bufferedCopier
}

// NewCopier creates a new Copier
func NewCopier(cid string, srcs map[string]io.Reader, dst Logger) *Copier {
	return &Copier{
		cid:        cid,
		srcs:       srcs,
		dst:        dst,
		closed:     make(chan struct{}),
		copierFunc: newScannerCopier,
	}
}

// Run starts logs copying
func (c *Copier) Run() {
	for src, w := range c.srcs {
		c.copyJobs.Add(1)
		go c.copySrc(src, w)
	}
}

func (c *Copier) copySrc(name string, src io.Reader) {
	defer c.copyJobs.Done()
	bf := c.copierFunc(src)

	for bf.Scan() {
		select {
		case <-c.closed:
			return
		default:
			line, err := bf.Bytes()

			if err == nil || len(line) > 0 {
				if logErr := c.dst.Log(&Message{ContainerID: c.cid, Line: line, Source: name, Timestamp: time.Now().UTC()}); logErr != nil {
					logrus.Errorf("Failed to log msg %q for logger %s: %s", line, c.dst.Name(), logErr)
				}
			}

			if err != nil {
				if err != io.EOF {
					logrus.Errorf("Error scanning log stream: %s", err)
				}
				return
			}
		}
	}
}

// Wait waits until all copying is done
func (c *Copier) Wait() {
	c.copyJobs.Wait()
}

// Close closes the copier
func (c *Copier) Close() {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
}

type readerCopier struct {
	r *bufio.Reader
}

func (s readerCopier) Scan() bool {
	return true
}
func (s readerCopier) Bytes() ([]byte, error) {
	line, err := s.r.ReadBytes('\n')
	line = bytes.TrimSuffix(line, []byte{'\n'})
	return line, err
}

func newReaderCopier(src io.Reader) bufferedCopier {
	return readerCopier{bufio.NewReader(src)}
}

type scannerCopier struct {
	s *bufio.Scanner
}

func (s scannerCopier) Scan() bool {
	return s.s.Scan()
}
func (s scannerCopier) Bytes() ([]byte, error) {
	return s.s.Bytes(), s.s.Err()
}

func newScannerCopier(src io.Reader) bufferedCopier {
	scanner := bufio.NewScanner(src)
	scanner.Split(splitLines)
	return scannerCopier{scanner}
}

// splitLines is a split function for a Scanner that returns each line of
// text, stripped of any trailing end-of-line marker. The returned line may
// be empty. The end-of-line marker is one optional carriage return followed
// by one mandatory newline. In regular expression notation, it is `\r?\n`.
// The last non-empty line of input will be returned even if it has no
// newline.
// It splits the data in several parts if its size is bigger than `maxScanBufferSize` to avoid
// infinite buffers that don't include any end-of-line marker.
//
// This function is a fork of bufio.ScanLines https://golang.org/pkg/bufio/#ScanLines.
// It adds an extra check to not buffer more data than a copier can handle.
func splitLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, dropCR(data[0:i]), nil
	}

	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), dropCR(data), nil
	}

	// split data if the buffer is too big.
	if len(data) > maxScanBufferSize {
		return maxScanBufferSize + 1, dropCR(data[0:maxScanBufferSize]), nil
	}

	// Request more data.
	return 0, nil, nil
}

// dropCR drops a terminal \r from the data.
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}
