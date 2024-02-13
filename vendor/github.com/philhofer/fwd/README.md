
# fwd

[![Go Reference](https://pkg.go.dev/badge/github.com/philhofer/fwd.svg)](https://pkg.go.dev/github.com/philhofer/fwd)


`import "github.com/philhofer/fwd"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
Package fwd provides a buffered reader
and writer. Each has methods that help improve
the encoding/decoding performance of some binary
protocols.

The `Writer` and `Reader` type provide similar
functionality to their counterparts in `bufio`, plus
a few extra utility methods that simplify read-ahead
and write-ahead. I wrote this package to improve serialization
performance for [github.com/tinylib/msgp](https://github.com/tinylib/msgp),
where it provided about a 2x speedup over `bufio` for certain
workloads. However, care must be taken to understand the semantics of the
extra methods provided by this package, as they allow
the user to access and manipulate the buffer memory
directly.

The extra methods for `fwd.Reader` are `Peek`, `Skip`
and `Next`. `(*fwd.Reader).Peek`, unlike `(*bufio.Reader).Peek`,
will re-allocate the read buffer in order to accommodate arbitrarily
large read-ahead. `(*fwd.Reader).Skip` skips the next `n` bytes
in the stream, and uses the `io.Seeker` interface if the underlying
stream implements it. `(*fwd.Reader).Next` returns a slice pointing
to the next `n` bytes in the read buffer (like `Peek`), but also
increments the read position. This allows users to process streams
in arbitrary block sizes without having to manage appropriately-sized
slices. Additionally, obviating the need to copy the data from the
buffer to another location in memory can improve performance dramatically
in CPU-bound applications.

`fwd.Writer` only has one extra method, which is `(*fwd.Writer).Next`, which
returns a slice pointing to the next `n` bytes of the writer, and increments
the write position by the length of the returned slice. This allows users
to write directly to the end of the buffer.




## <a name="pkg-index">Index</a>
* [Constants](#pkg-constants)
* [type Reader](#Reader)
  * [func NewReader(r io.Reader) *Reader](#NewReader)
  * [func NewReaderBuf(r io.Reader, buf []byte) *Reader](#NewReaderBuf)
  * [func NewReaderSize(r io.Reader, n int) *Reader](#NewReaderSize)
  * [func (r *Reader) BufferSize() int](#Reader.BufferSize)
  * [func (r *Reader) Buffered() int](#Reader.Buffered)
  * [func (r *Reader) Next(n int) ([]byte, error)](#Reader.Next)
  * [func (r *Reader) Peek(n int) ([]byte, error)](#Reader.Peek)
  * [func (r *Reader) Read(b []byte) (int, error)](#Reader.Read)
  * [func (r *Reader) ReadByte() (byte, error)](#Reader.ReadByte)
  * [func (r *Reader) ReadFull(b []byte) (int, error)](#Reader.ReadFull)
  * [func (r *Reader) Reset(rd io.Reader)](#Reader.Reset)
  * [func (r *Reader) Skip(n int) (int, error)](#Reader.Skip)
  * [func (r *Reader) WriteTo(w io.Writer) (int64, error)](#Reader.WriteTo)
* [type Writer](#Writer)
  * [func NewWriter(w io.Writer) *Writer](#NewWriter)
  * [func NewWriterBuf(w io.Writer, buf []byte) *Writer](#NewWriterBuf)
  * [func NewWriterSize(w io.Writer, n int) *Writer](#NewWriterSize)
  * [func (w *Writer) BufferSize() int](#Writer.BufferSize)
  * [func (w *Writer) Buffered() int](#Writer.Buffered)
  * [func (w *Writer) Flush() error](#Writer.Flush)
  * [func (w *Writer) Next(n int) ([]byte, error)](#Writer.Next)
  * [func (w *Writer) ReadFrom(r io.Reader) (int64, error)](#Writer.ReadFrom)
  * [func (w *Writer) Write(p []byte) (int, error)](#Writer.Write)
  * [func (w *Writer) WriteByte(b byte) error](#Writer.WriteByte)
  * [func (w *Writer) WriteString(s string) (int, error)](#Writer.WriteString)


## <a name="pkg-constants">Constants</a>
``` go
const (
    // DefaultReaderSize is the default size of the read buffer
    DefaultReaderSize = 2048
)
```
``` go
const (
    // DefaultWriterSize is the
    // default write buffer size.
    DefaultWriterSize = 2048
)
```



## type Reader
``` go
type Reader struct {
    // contains filtered or unexported fields
}
```
Reader is a buffered look-ahead reader









### func NewReader
``` go
func NewReader(r io.Reader) *Reader
```
NewReader returns a new *Reader that reads from 'r'


### func NewReaderSize
``` go
func NewReaderSize(r io.Reader, n int) *Reader
```
NewReaderSize returns a new *Reader that
reads from 'r' and has a buffer size 'n'




### func (\*Reader) BufferSize
``` go
func (r *Reader) BufferSize() int
```
BufferSize returns the total size of the buffer



### func (\*Reader) Buffered
``` go
func (r *Reader) Buffered() int
```
Buffered returns the number of bytes currently in the buffer



### func (\*Reader) Next
``` go
func (r *Reader) Next(n int) ([]byte, error)
```
Next returns the next 'n' bytes in the stream.
Unlike Peek, Next advances the reader position.
The returned bytes point to the same
data as the buffer, so the slice is
only valid until the next reader method call.
An EOF is considered an unexpected error.
If an the returned slice is less than the
length asked for, an error will be returned,
and the reader position will not be incremented.



### <a name="Reader.Peek">func</a> (\*Reader) Peek
``` go
func (r *Reader) Peek(n int) ([]byte, error)
```
Peek returns the next 'n' buffered bytes,
reading from the underlying reader if necessary.
It will only return a slice shorter than 'n' bytes
if it also returns an error. Peek does not advance
the reader. EOF errors are *not* returned as
io.ErrUnexpectedEOF.



### <a name="Reader.Read">func</a> (\*Reader) Read
``` go
func (r *Reader) Read(b []byte) (int, error)
```
Read implements `io.Reader`.



### <a name="Reader.ReadByte">func</a> (\*Reader) ReadByte
``` go
func (r *Reader) ReadByte() (byte, error)
```
ReadByte implements `io.ByteReader`.



### <a name="Reader.ReadFull">func</a> (\*Reader) ReadFull
``` go
func (r *Reader) ReadFull(b []byte) (int, error)
```
ReadFull attempts to read len(b) bytes into
'b'. It returns the number of bytes read into
'b', and an error if it does not return len(b).
EOF is considered an unexpected error.



### <a name="Reader.Reset">func</a> (\*Reader) Reset
``` go
func (r *Reader) Reset(rd io.Reader)
```
Reset resets the underlying reader
and the read buffer.



### <a name="Reader.Skip">func</a> (\*Reader) Skip
``` go
func (r *Reader) Skip(n int) (int, error)
```
Skip moves the reader forward 'n' bytes.
Returns the number of bytes skipped and any
errors encountered. It is analogous to Seek(n, 1).
If the underlying reader implements io.Seeker, then
that method will be used to skip forward.

If the reader encounters
an EOF before skipping 'n' bytes, it
returns `io.ErrUnexpectedEOF`. If the
underlying reader implements `io.Seeker`, then
those rules apply instead. (Many implementations
will not return `io.EOF` until the next call
to Read).




### <a name="Reader.WriteTo">func</a> (\*Reader) WriteTo
``` go
func (r *Reader) WriteTo(w io.Writer) (int64, error)
```
WriteTo implements `io.WriterTo`.




## <a name="Writer">type</a> Writer
``` go
type Writer struct {
    // contains filtered or unexported fields
}

```
Writer is a buffered writer







### <a name="NewWriter">func</a> NewWriter
``` go
func NewWriter(w io.Writer) *Writer
```
NewWriter returns a new writer
that writes to 'w' and has a buffer
that is `DefaultWriterSize` bytes.


### <a name="NewWriterBuf">func</a> NewWriterBuf
``` go
func NewWriterBuf(w io.Writer, buf []byte) *Writer
```
NewWriterBuf returns a new writer
that writes to 'w' and has 'buf' as a buffer.
'buf' is not used when has smaller capacity than 18,
custom buffer is allocated instead.


### <a name="NewWriterSize">func</a> NewWriterSize
``` go
func NewWriterSize(w io.Writer, n int) *Writer
```
NewWriterSize returns a new writer that
writes to 'w' and has a buffer size 'n'.

### <a name="Writer.BufferSize">func</a> (\*Writer) BufferSize
``` go
func (w *Writer) BufferSize() int
```
BufferSize returns the maximum size of the buffer.



### <a name="Writer.Buffered">func</a> (\*Writer) Buffered
``` go
func (w *Writer) Buffered() int
```
Buffered returns the number of buffered bytes
in the reader.



### <a name="Writer.Flush">func</a> (\*Writer) Flush
``` go
func (w *Writer) Flush() error
```
Flush flushes any buffered bytes
to the underlying writer.



### <a name="Writer.Next">func</a> (\*Writer) Next
``` go
func (w *Writer) Next(n int) ([]byte, error)
```
Next returns the next 'n' free bytes
in the write buffer, flushing the writer
as necessary. Next will return `io.ErrShortBuffer`
if 'n' is greater than the size of the write buffer.
Calls to 'next' increment the write position by
the size of the returned buffer.



### <a name="Writer.ReadFrom">func</a> (\*Writer) ReadFrom
``` go
func (w *Writer) ReadFrom(r io.Reader) (int64, error)
```
ReadFrom implements `io.ReaderFrom`



### <a name="Writer.Write">func</a> (\*Writer) Write
``` go
func (w *Writer) Write(p []byte) (int, error)
```
Write implements `io.Writer`



### <a name="Writer.WriteByte">func</a> (\*Writer) WriteByte
``` go
func (w *Writer) WriteByte(b byte) error
```
WriteByte implements `io.ByteWriter`



### <a name="Writer.WriteString">func</a> (\*Writer) WriteString
``` go
func (w *Writer) WriteString(s string) (int, error)
```
WriteString is analogous to Write, but it takes a string.








- - -
Generated by [godoc2md](https://github.com/davecheney/godoc2md)
