// Copyright 2015 LinkedIn Corp. Licensed under the Apache License,
// Version 2.0 (the "License"); you may not use this file except in
// compliance with the License.  You may obtain a copy of the License
// at http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.Copyright [201X] LinkedIn Corp. Licensed under the Apache
// License, Version 2.0 (the "License"); you may not use this file
// except in compliance with the License.  You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.

package goavro

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"

	"github.com/golang/snappy"
)

// ErrReaderInit is returned when the encoder encounters an error.
type ErrReaderInit struct {
	Message string
	Err     error
}

func (e ErrReaderInit) Error() string {
	if e.Err == nil {
		return "cannot build " + e.Message
	}
	return "cannot build " + e.Message + ": " + e.Err.Error()
}

func newReaderInitError(a ...interface{}) *ErrReaderInit {
	var err error
	var format, message string
	var ok bool
	if len(a) == 0 {
		return &ErrReaderInit{"cannot create reader: no reason given", nil}
	}
	// if last item is error: save it
	if err, ok = a[len(a)-1].(error); ok {
		a = a[:len(a)-1] // pop it
	}
	// if items left, first ought to be format string
	if len(a) > 0 {
		if format, ok = a[0].(string); ok {
			a = a[1:] // unshift
			message = fmt.Sprintf(format, a...)
		}
	}
	return &ErrReaderInit{message, err}
}

// ErrReaderBlockCount is returned when a reader detects an error
// while attempting to read the block count and block size.
type ErrReaderBlockCount struct {
	Err error
}

func (e *ErrReaderBlockCount) Error() string {
	return "cannot read block count and size: " + e.Err.Error()
}

// ReaderSetter functions are those those which are used to instantiate
// a new Reader.
type ReaderSetter func(*Reader) error

// BufferFromReader wraps the specified `io.Reader` using a
// `bufio.Reader` to read from a file.
func BufferFromReader(r io.Reader) ReaderSetter {
	return func(fr *Reader) error {
		fr.r = bufio.NewReader(r)
		return nil
	}
}

// FromReader specifies the `io.Reader` to use when reading a file.
func FromReader(r io.Reader) ReaderSetter {
	return func(fr *Reader) error {
		fr.r = r
		return nil
	}
}

// Reader structure contains data necessary to read Avro files.
type Reader struct {
	CompressionCodec string
	DataSchema       string
	Sync             []byte
	dataCodec        Codec
	datum            Datum
	deblocked        chan Datum
	err              error
	r                io.Reader
}

// NewReader returns a object to read data from an io.Reader using the
// Avro Object Container Files format.
//
//     func main() {
//         conn, err := net.Dial("tcp", "127.0.0.1:8080")
//         if err != nil {
//             log.Fatal(err)
//         }
//         fr, err := goavro.NewReader(goavro.FromReader(conn))
//         if err != nil {
//             log.Fatal("cannot create Reader: ", err)
//         }
//         defer func() {
//             if err := fr.Close(); err != nil {
//                 log.Fatal(err)
//             }
//         }()
//
//         for fr.Scan() {
//             datum, err := fr.Read()
//             if err != nil {
//                 log.Println("cannot read datum: ", err)
//                 continue
//             }
//             fmt.Println("RECORD: ", datum)
//         }
//     }
func NewReader(setters ...ReaderSetter) (*Reader, error) {
	var err error
	fr := &Reader{}
	for _, setter := range setters {
		err = setter(fr)
		if err != nil {
			return nil, newReaderInitError(err)
		}
	}
	if fr.r == nil {
		return nil, newReaderInitError("must specify io.Reader")
	}
	// read in header information and use it to initialize Reader
	magic := make([]byte, 4)
	_, err = fr.r.Read(magic)
	if err != nil {
		return nil, newReaderInitError("cannot read magic number", err)
	}
	if bytes.Compare(magic, []byte(magicBytes)) != 0 {
		return nil, &ErrReaderInit{Message: "invalid magic number: " + string(magic)}
	}
	meta, err := decodeHeaderMetadata(fr.r)
	if err != nil {
		return nil, newReaderInitError("cannot read header metadata", err)
	}
	fr.CompressionCodec, err = getHeaderString("avro.codec", meta)
	if err != nil {
		fr.CompressionCodec = CompressionNull
	}
	if !IsCompressionCodecSupported(fr.CompressionCodec) {
		return nil, newReaderInitError("unsupported codec: %s", fr.CompressionCodec)
	}
	fr.DataSchema, err = getHeaderString("avro.schema", meta)
	if err != nil {
		return nil, newReaderInitError("cannot read header metadata", err)
	}
	if fr.dataCodec, err = NewCodec(fr.DataSchema); err != nil {
		return nil, newReaderInitError("cannot compile schema", err)
	}
	fr.Sync = make([]byte, syncLength)
	if _, err = fr.r.Read(fr.Sync); err != nil {
		return nil, newReaderInitError("cannot read sync marker", err)
	}
	// setup reading pipeline
	toDecompress := make(chan *readerBlock)
	toDecode := make(chan *readerBlock)
	fr.deblocked = make(chan Datum)
	go read(fr, toDecompress)
	go decompress(fr, toDecompress, toDecode)
	go decode(fr, toDecode)
	return fr, nil
}

// Close releases resources and returns any Reader errors.
func (fr *Reader) Close() error {
	return fr.err
}

// Scan returns true if more data is ready to be read.
func (fr *Reader) Scan() bool {
	var ok bool
	fr.datum, ok = <-fr.deblocked
	return ok
}

// Read returns the next element from the Reader.
func (fr *Reader) Read() (interface{}, error) {
	return fr.datum.Value, fr.datum.Err
}

func decodeHeaderMetadata(r io.Reader) (map[string]interface{}, error) {
	md, err := metadataCodec.Decode(r)
	if err != nil {
		return nil, err
	}
	return md.(map[string]interface{}), nil
}

func getHeaderString(someKey string, header map[string]interface{}) (string, error) {
	v, ok := header[someKey]
	if !ok {
		return "", fmt.Errorf("header ought to have %v key", someKey)
	}
	return string(v.([]byte)), nil
}

type readerBlock struct {
	datumCount int
	err        error
	r          io.Reader
}

// ErrReader is returned when the reader encounters an error.
type ErrReader struct {
	Message string
	Err     error
}

func (e ErrReader) Error() string {
	if e.Err == nil {
		return "cannot read from reader: " + e.Message
	}
	return "cannot read from reader: " + e.Message + ": " + e.Err.Error()
}

func newReaderError(a ...interface{}) *ErrReader {
	var err error
	var format, message string
	var ok bool
	if len(a) == 0 {
		return &ErrReader{"no reason given", nil}
	}
	// if last item is error: save it
	if err, ok = a[len(a)-1].(error); ok {
		a = a[:len(a)-1] // pop it
	}
	// if items left, first ought to be format string
	if len(a) > 0 {
		if format, ok = a[0].(string); ok {
			a = a[1:] // unshift
			message = fmt.Sprintf(format, a...)
		}
	}
	return &ErrReader{message, err}
}

func read(fr *Reader, toDecompress chan<- *readerBlock) {
	// NOTE: these variables created outside loop to reduce churn
	var lr io.Reader
	var bits []byte
	sync := make([]byte, syncLength)

	blockCount, blockSize, err := readBlockCountAndSize(fr.r)
	if err != nil {
		fr.err = err
		blockCount = 0
	}
	for blockCount != 0 {
		lr = io.LimitReader(fr.r, int64(blockSize))
		if bits, err = ioutil.ReadAll(lr); err != nil {
			err = newReaderError("cannot read block", err)
			break
		}
		toDecompress <- &readerBlock{datumCount: blockCount, r: bytes.NewReader(bits)}
		if _, fr.err = fr.r.Read(sync); fr.err != nil {
			err = newReaderError("cannot read sync marker", fr.err)
			break
		}
		if !bytes.Equal(fr.Sync, sync) {
			fr.err = newReaderError(fmt.Sprintf("sync marker mismatch: %#v != %#v", sync, fr.Sync))
			break
		}
		if blockCount, blockSize, fr.err = readBlockCountAndSize(fr.r); fr.err != nil {
			break
		}
	}
	close(toDecompress)
}

func readBlockCountAndSize(r io.Reader) (int, int, error) {
	bc, err := longCodec.Decode(r)
	if err != nil {
		if ed, ok := err.(*ErrDecoder); ok && ed.Err == io.EOF {
			return 0, 0, nil // we're done
		}
		return 0, 0, &ErrReaderBlockCount{err}
	}
	bs, err := longCodec.Decode(r)
	if err != nil {
		return 0, 0, &ErrReaderBlockCount{err}
	}
	return int(bc.(int64)), int(bs.(int64)), nil
}

func decompress(fr *Reader, toDecompress <-chan *readerBlock, toDecode chan<- *readerBlock) {
	switch fr.CompressionCodec {
	case CompressionNull:
		for block := range toDecompress {
			toDecode <- block
		}

	case CompressionDeflate:
		var rc io.ReadCloser
		var bits []byte
		for block := range toDecompress {
			rc = flate.NewReader(block.r)
			bits, block.err = ioutil.ReadAll(rc)
			if block.err != nil {
				block.err = newReaderError("cannot read from deflate", block.err)
				toDecode <- block
				_ = rc.Close() // already have the read error; ignore the close error
				continue
			}
			block.err = rc.Close()
			if block.err != nil {
				block.err = newReaderError("cannot close deflate", block.err)
				toDecode <- block
				continue
			}
			block.r = bytes.NewReader(bits)
			toDecode <- block
		}

	case CompressionSnappy:
		var (
			src, dst []byte
			crc      uint32
		)
		for block := range toDecompress {
			src, block.err = ioutil.ReadAll(block.r)
			if block.err != nil {
				block.err = newReaderError("cannot read", block.err)
				toDecode <- block
				continue
			}
			index := len(src) - 4 // last 4 bytes is crc32 of decoded blob

			dst, block.err = snappy.Decode(nil, src[:index])
			if block.err != nil {
				block.err = newReaderError("cannot decompress", block.err)
				toDecode <- block
				continue
			}

			block.err = binary.Read(bytes.NewReader(src[index:index+4]), binary.BigEndian, &crc)
			if block.err != nil {
				block.err = newReaderError("failed to read crc checksum after snappy block", block.err)
				toDecode <- block
				continue
			}

			if crc != crc32.ChecksumIEEE(dst) {
				block.err = newReaderError("snappy crc checksum mismatch", block.err)
				toDecode <- block
				continue
			}

			block.r = bytes.NewReader(dst)
			toDecode <- block
		}
	}
	close(toDecode)
}

func decode(fr *Reader, toDecode <-chan *readerBlock) {
decodeLoop:
	for block := range toDecode {
		for i := 0; i < block.datumCount; i++ {
			var datum Datum
			datum.Value, datum.Err = fr.dataCodec.Decode(block.r)
			if datum.Value == nil && datum.Err == nil {
				break decodeLoop
			}
			fr.deblocked <- datum
		}
	}
	close(fr.deblocked)
}
