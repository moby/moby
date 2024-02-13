package msgp

import (
	"errors"
	"io"
	"math"
	"reflect"
	"sync"
	"time"
)

const (
	// min buffer size for the writer
	minWriterSize = 18
)

// Sizer is an interface implemented
// by types that can estimate their
// size when MessagePack encoded.
// This interface is optional, but
// encoding/marshaling implementations
// may use this as a way to pre-allocate
// memory for serialization.
type Sizer interface {
	Msgsize() int
}

var (
	// Nowhere is an io.Writer to nowhere
	Nowhere io.Writer = nwhere{}

	btsType    = reflect.TypeOf(([]byte)(nil))
	writerPool = sync.Pool{
		New: func() interface{} {
			return &Writer{buf: make([]byte, 2048)}
		},
	}
)

func popWriter(w io.Writer) *Writer {
	wr := writerPool.Get().(*Writer)
	wr.Reset(w)
	return wr
}

func pushWriter(wr *Writer) {
	wr.w = nil
	wr.wloc = 0
	writerPool.Put(wr)
}

// freeW frees a writer for use
// by other processes. It is not necessary
// to call freeW on a writer. However, maintaining
// a reference to a *Writer after calling freeW on
// it will cause undefined behavior.
func freeW(w *Writer) { pushWriter(w) }

// Require ensures that cap(old)-len(old) >= extra.
func Require(old []byte, extra int) []byte {
	l := len(old)
	c := cap(old)
	r := l + extra
	if c >= r {
		return old
	} else if l == 0 {
		return make([]byte, 0, extra)
	}
	// the new size is the greater
	// of double the old capacity
	// and the sum of the old length
	// and the number of new bytes
	// necessary.
	c <<= 1
	if c < r {
		c = r
	}
	n := make([]byte, l, c)
	copy(n, old)
	return n
}

// nowhere writer
type nwhere struct{}

func (n nwhere) Write(p []byte) (int, error) { return len(p), nil }

// Marshaler is the interface implemented
// by types that know how to marshal themselves
// as MessagePack. MarshalMsg appends the marshalled
// form of the object to the provided
// byte slice, returning the extended
// slice and any errors encountered.
type Marshaler interface {
	MarshalMsg([]byte) ([]byte, error)
}

// Encodable is the interface implemented
// by types that know how to write themselves
// as MessagePack using a *msgp.Writer.
type Encodable interface {
	EncodeMsg(*Writer) error
}

// Writer is a buffered writer
// that can be used to write
// MessagePack objects to an io.Writer.
// You must call *Writer.Flush() in order
// to flush all of the buffered data
// to the underlying writer.
type Writer struct {
	w    io.Writer
	buf  []byte
	wloc int
}

// NewWriter returns a new *Writer.
func NewWriter(w io.Writer) *Writer {
	if wr, ok := w.(*Writer); ok {
		return wr
	}
	return popWriter(w)
}

// NewWriterSize returns a writer with a custom buffer size.
func NewWriterSize(w io.Writer, sz int) *Writer {
	// we must be able to require() 'minWriterSize'
	// contiguous bytes, so that is the
	// practical minimum buffer size
	if sz < minWriterSize {
		sz = minWriterSize
	}
	buf := make([]byte, sz)
	return NewWriterBuf(w, buf)
}

// NewWriterBuf returns a writer with a provided buffer.
// 'buf' is not used when the capacity is smaller than 18,
// custom buffer is allocated instead.
func NewWriterBuf(w io.Writer, buf []byte) *Writer {
	if cap(buf) < minWriterSize {
		buf = make([]byte, minWriterSize)
	}
	buf = buf[:cap(buf)]
	return &Writer{
		w:   w,
		buf: buf,
	}
}

// Encode encodes an Encodable to an io.Writer.
func Encode(w io.Writer, e Encodable) error {
	wr := NewWriter(w)
	err := e.EncodeMsg(wr)
	if err == nil {
		err = wr.Flush()
	}
	freeW(wr)
	return err
}

func (mw *Writer) flush() error {
	if mw.wloc == 0 {
		return nil
	}
	n, err := mw.w.Write(mw.buf[:mw.wloc])
	if err != nil {
		if n > 0 {
			mw.wloc = copy(mw.buf, mw.buf[n:mw.wloc])
		}
		return err
	}
	mw.wloc = 0
	return nil
}

// Flush flushes all of the buffered
// data to the underlying writer.
func (mw *Writer) Flush() error { return mw.flush() }

// Buffered returns the number bytes in the write buffer
func (mw *Writer) Buffered() int { return len(mw.buf) - mw.wloc }

func (mw *Writer) avail() int { return len(mw.buf) - mw.wloc }

func (mw *Writer) bufsize() int { return len(mw.buf) }

// NOTE: this should only be called with
// a number that is guaranteed to be less than
// len(mw.buf). typically, it is called with a constant.
//
// NOTE: this is a hot code path
func (mw *Writer) require(n int) (int, error) {
	c := len(mw.buf)
	wl := mw.wloc
	if c-wl < n {
		if err := mw.flush(); err != nil {
			return 0, err
		}
		wl = mw.wloc
	}
	mw.wloc += n
	return wl, nil
}

func (mw *Writer) Append(b ...byte) error {
	if mw.avail() < len(b) {
		err := mw.flush()
		if err != nil {
			return err
		}
	}
	mw.wloc += copy(mw.buf[mw.wloc:], b)
	return nil
}

// push one byte onto the buffer
//
// NOTE: this is a hot code path
func (mw *Writer) push(b byte) error {
	if mw.wloc == len(mw.buf) {
		if err := mw.flush(); err != nil {
			return err
		}
	}
	mw.buf[mw.wloc] = b
	mw.wloc++
	return nil
}

func (mw *Writer) prefix8(b byte, u uint8) error {
	const need = 2
	if len(mw.buf)-mw.wloc < need {
		if err := mw.flush(); err != nil {
			return err
		}
	}
	prefixu8(mw.buf[mw.wloc:], b, u)
	mw.wloc += need
	return nil
}

func (mw *Writer) prefix16(b byte, u uint16) error {
	const need = 3
	if len(mw.buf)-mw.wloc < need {
		if err := mw.flush(); err != nil {
			return err
		}
	}
	prefixu16(mw.buf[mw.wloc:], b, u)
	mw.wloc += need
	return nil
}

func (mw *Writer) prefix32(b byte, u uint32) error {
	const need = 5
	if len(mw.buf)-mw.wloc < need {
		if err := mw.flush(); err != nil {
			return err
		}
	}
	prefixu32(mw.buf[mw.wloc:], b, u)
	mw.wloc += need
	return nil
}

func (mw *Writer) prefix64(b byte, u uint64) error {
	const need = 9
	if len(mw.buf)-mw.wloc < need {
		if err := mw.flush(); err != nil {
			return err
		}
	}
	prefixu64(mw.buf[mw.wloc:], b, u)
	mw.wloc += need
	return nil
}

// Write implements io.Writer, and writes
// data directly to the buffer.
func (mw *Writer) Write(p []byte) (int, error) {
	l := len(p)
	if mw.avail() < l {
		if err := mw.flush(); err != nil {
			return 0, err
		}
		if l > len(mw.buf) {
			return mw.w.Write(p)
		}
	}
	mw.wloc += copy(mw.buf[mw.wloc:], p)
	return l, nil
}

// implements io.WriteString
func (mw *Writer) writeString(s string) error {
	l := len(s)
	if mw.avail() < l {
		if err := mw.flush(); err != nil {
			return err
		}
		if l > len(mw.buf) {
			_, err := io.WriteString(mw.w, s)
			return err
		}
	}
	mw.wloc += copy(mw.buf[mw.wloc:], s)
	return nil
}

// Reset changes the underlying writer used by the Writer
func (mw *Writer) Reset(w io.Writer) {
	mw.buf = mw.buf[:cap(mw.buf)]
	mw.w = w
	mw.wloc = 0
}

// WriteMapHeader writes a map header of the given
// size to the writer
func (mw *Writer) WriteMapHeader(sz uint32) error {
	switch {
	case sz <= 15:
		return mw.push(wfixmap(uint8(sz)))
	case sz <= math.MaxUint16:
		return mw.prefix16(mmap16, uint16(sz))
	default:
		return mw.prefix32(mmap32, sz)
	}
}

// WriteArrayHeader writes an array header of the
// given size to the writer
func (mw *Writer) WriteArrayHeader(sz uint32) error {
	switch {
	case sz <= 15:
		return mw.push(wfixarray(uint8(sz)))
	case sz <= math.MaxUint16:
		return mw.prefix16(marray16, uint16(sz))
	default:
		return mw.prefix32(marray32, sz)
	}
}

// WriteNil writes a nil byte to the buffer
func (mw *Writer) WriteNil() error {
	return mw.push(mnil)
}

// WriteFloat64 writes a float64 to the writer
func (mw *Writer) WriteFloat64(f float64) error {
	return mw.prefix64(mfloat64, math.Float64bits(f))
}

// WriteFloat32 writes a float32 to the writer
func (mw *Writer) WriteFloat32(f float32) error {
	return mw.prefix32(mfloat32, math.Float32bits(f))
}

// WriteDuration writes a time.Duration to the writer
func (mw *Writer) WriteDuration(d time.Duration) error {
	return mw.WriteInt64(int64(d))
}

// WriteInt64 writes an int64 to the writer
func (mw *Writer) WriteInt64(i int64) error {
	if i >= 0 {
		switch {
		case i <= math.MaxInt8:
			return mw.push(wfixint(uint8(i)))
		case i <= math.MaxInt16:
			return mw.prefix16(mint16, uint16(i))
		case i <= math.MaxInt32:
			return mw.prefix32(mint32, uint32(i))
		default:
			return mw.prefix64(mint64, uint64(i))
		}
	}
	switch {
	case i >= -32:
		return mw.push(wnfixint(int8(i)))
	case i >= math.MinInt8:
		return mw.prefix8(mint8, uint8(i))
	case i >= math.MinInt16:
		return mw.prefix16(mint16, uint16(i))
	case i >= math.MinInt32:
		return mw.prefix32(mint32, uint32(i))
	default:
		return mw.prefix64(mint64, uint64(i))
	}
}

// WriteInt8 writes an int8 to the writer
func (mw *Writer) WriteInt8(i int8) error { return mw.WriteInt64(int64(i)) }

// WriteInt16 writes an int16 to the writer
func (mw *Writer) WriteInt16(i int16) error { return mw.WriteInt64(int64(i)) }

// WriteInt32 writes an int32 to the writer
func (mw *Writer) WriteInt32(i int32) error { return mw.WriteInt64(int64(i)) }

// WriteInt writes an int to the writer
func (mw *Writer) WriteInt(i int) error { return mw.WriteInt64(int64(i)) }

// WriteUint64 writes a uint64 to the writer
func (mw *Writer) WriteUint64(u uint64) error {
	switch {
	case u <= (1<<7)-1:
		return mw.push(wfixint(uint8(u)))
	case u <= math.MaxUint8:
		return mw.prefix8(muint8, uint8(u))
	case u <= math.MaxUint16:
		return mw.prefix16(muint16, uint16(u))
	case u <= math.MaxUint32:
		return mw.prefix32(muint32, uint32(u))
	default:
		return mw.prefix64(muint64, u)
	}
}

// WriteByte is analogous to WriteUint8
func (mw *Writer) WriteByte(u byte) error { return mw.WriteUint8(uint8(u)) }

// WriteUint8 writes a uint8 to the writer
func (mw *Writer) WriteUint8(u uint8) error { return mw.WriteUint64(uint64(u)) }

// WriteUint16 writes a uint16 to the writer
func (mw *Writer) WriteUint16(u uint16) error { return mw.WriteUint64(uint64(u)) }

// WriteUint32 writes a uint32 to the writer
func (mw *Writer) WriteUint32(u uint32) error { return mw.WriteUint64(uint64(u)) }

// WriteUint writes a uint to the writer
func (mw *Writer) WriteUint(u uint) error { return mw.WriteUint64(uint64(u)) }

// WriteBytes writes binary as 'bin' to the writer
func (mw *Writer) WriteBytes(b []byte) error {
	sz := uint32(len(b))
	var err error
	switch {
	case sz <= math.MaxUint8:
		err = mw.prefix8(mbin8, uint8(sz))
	case sz <= math.MaxUint16:
		err = mw.prefix16(mbin16, uint16(sz))
	default:
		err = mw.prefix32(mbin32, sz)
	}
	if err != nil {
		return err
	}
	_, err = mw.Write(b)
	return err
}

// WriteBytesHeader writes just the size header
// of a MessagePack 'bin' object. The user is responsible
// for then writing 'sz' more bytes into the stream.
func (mw *Writer) WriteBytesHeader(sz uint32) error {
	switch {
	case sz <= math.MaxUint8:
		return mw.prefix8(mbin8, uint8(sz))
	case sz <= math.MaxUint16:
		return mw.prefix16(mbin16, uint16(sz))
	default:
		return mw.prefix32(mbin32, sz)
	}
}

// WriteBool writes a bool to the writer
func (mw *Writer) WriteBool(b bool) error {
	if b {
		return mw.push(mtrue)
	}
	return mw.push(mfalse)
}

// WriteString writes a messagepack string to the writer.
// (This is NOT an implementation of io.StringWriter)
func (mw *Writer) WriteString(s string) error {
	sz := uint32(len(s))
	var err error
	switch {
	case sz <= 31:
		err = mw.push(wfixstr(uint8(sz)))
	case sz <= math.MaxUint8:
		err = mw.prefix8(mstr8, uint8(sz))
	case sz <= math.MaxUint16:
		err = mw.prefix16(mstr16, uint16(sz))
	default:
		err = mw.prefix32(mstr32, sz)
	}
	if err != nil {
		return err
	}
	return mw.writeString(s)
}

// WriteStringHeader writes just the string size
// header of a MessagePack 'str' object. The user
// is responsible for writing 'sz' more valid UTF-8
// bytes to the stream.
func (mw *Writer) WriteStringHeader(sz uint32) error {
	switch {
	case sz <= 31:
		return mw.push(wfixstr(uint8(sz)))
	case sz <= math.MaxUint8:
		return mw.prefix8(mstr8, uint8(sz))
	case sz <= math.MaxUint16:
		return mw.prefix16(mstr16, uint16(sz))
	default:
		return mw.prefix32(mstr32, sz)
	}
}

// WriteStringFromBytes writes a 'str' object
// from a []byte.
func (mw *Writer) WriteStringFromBytes(str []byte) error {
	sz := uint32(len(str))
	var err error
	switch {
	case sz <= 31:
		err = mw.push(wfixstr(uint8(sz)))
	case sz <= math.MaxUint8:
		err = mw.prefix8(mstr8, uint8(sz))
	case sz <= math.MaxUint16:
		err = mw.prefix16(mstr16, uint16(sz))
	default:
		err = mw.prefix32(mstr32, sz)
	}
	if err != nil {
		return err
	}
	_, err = mw.Write(str)
	return err
}

// WriteComplex64 writes a complex64 to the writer
func (mw *Writer) WriteComplex64(f complex64) error {
	o, err := mw.require(10)
	if err != nil {
		return err
	}
	mw.buf[o] = mfixext8
	mw.buf[o+1] = Complex64Extension
	big.PutUint32(mw.buf[o+2:], math.Float32bits(real(f)))
	big.PutUint32(mw.buf[o+6:], math.Float32bits(imag(f)))
	return nil
}

// WriteComplex128 writes a complex128 to the writer
func (mw *Writer) WriteComplex128(f complex128) error {
	o, err := mw.require(18)
	if err != nil {
		return err
	}
	mw.buf[o] = mfixext16
	mw.buf[o+1] = Complex128Extension
	big.PutUint64(mw.buf[o+2:], math.Float64bits(real(f)))
	big.PutUint64(mw.buf[o+10:], math.Float64bits(imag(f)))
	return nil
}

// WriteMapStrStr writes a map[string]string to the writer
func (mw *Writer) WriteMapStrStr(mp map[string]string) (err error) {
	err = mw.WriteMapHeader(uint32(len(mp)))
	if err != nil {
		return
	}
	for key, val := range mp {
		err = mw.WriteString(key)
		if err != nil {
			return
		}
		err = mw.WriteString(val)
		if err != nil {
			return
		}
	}
	return nil
}

// WriteMapStrIntf writes a map[string]interface to the writer
func (mw *Writer) WriteMapStrIntf(mp map[string]interface{}) (err error) {
	err = mw.WriteMapHeader(uint32(len(mp)))
	if err != nil {
		return
	}
	for key, val := range mp {
		err = mw.WriteString(key)
		if err != nil {
			return
		}
		err = mw.WriteIntf(val)
		if err != nil {
			return
		}
	}
	return
}

// WriteTime writes a time.Time object to the wire.
//
// Time is encoded as Unix time, which means that
// location (time zone) data is removed from the object.
// The encoded object itself is 12 bytes: 8 bytes for
// a big-endian 64-bit integer denoting seconds
// elapsed since "zero" Unix time, followed by 4 bytes
// for a big-endian 32-bit signed integer denoting
// the nanosecond offset of the time. This encoding
// is intended to ease portability across languages.
// (Note that this is *not* the standard time.Time
// binary encoding, because its implementation relies
// heavily on the internal representation used by the
// time package.)
func (mw *Writer) WriteTime(t time.Time) error {
	t = t.UTC()
	o, err := mw.require(15)
	if err != nil {
		return err
	}
	mw.buf[o] = mext8
	mw.buf[o+1] = 12
	mw.buf[o+2] = TimeExtension
	putUnix(mw.buf[o+3:], t.Unix(), int32(t.Nanosecond()))
	return nil
}

// WriteIntf writes the concrete type of 'v'.
// WriteIntf will error if 'v' is not one of the following:
//   - A bool, float, string, []byte, int, uint, or complex
//   - A map of supported types (with string keys)
//   - An array or slice of supported types
//   - A pointer to a supported type
//   - A type that satisfies the msgp.Encodable interface
//   - A type that satisfies the msgp.Extension interface
func (mw *Writer) WriteIntf(v interface{}) error {
	if v == nil {
		return mw.WriteNil()
	}
	switch v := v.(type) {

	// preferred interfaces

	case Encodable:
		return v.EncodeMsg(mw)
	case Extension:
		return mw.WriteExtension(v)

	// concrete types

	case bool:
		return mw.WriteBool(v)
	case float32:
		return mw.WriteFloat32(v)
	case float64:
		return mw.WriteFloat64(v)
	case complex64:
		return mw.WriteComplex64(v)
	case complex128:
		return mw.WriteComplex128(v)
	case uint8:
		return mw.WriteUint8(v)
	case uint16:
		return mw.WriteUint16(v)
	case uint32:
		return mw.WriteUint32(v)
	case uint64:
		return mw.WriteUint64(v)
	case uint:
		return mw.WriteUint(v)
	case int8:
		return mw.WriteInt8(v)
	case int16:
		return mw.WriteInt16(v)
	case int32:
		return mw.WriteInt32(v)
	case int64:
		return mw.WriteInt64(v)
	case int:
		return mw.WriteInt(v)
	case string:
		return mw.WriteString(v)
	case []byte:
		return mw.WriteBytes(v)
	case map[string]string:
		return mw.WriteMapStrStr(v)
	case map[string]interface{}:
		return mw.WriteMapStrIntf(v)
	case time.Time:
		return mw.WriteTime(v)
	case time.Duration:
		return mw.WriteDuration(v)
	}

	val := reflect.ValueOf(v)
	if !isSupported(val.Kind()) || !val.IsValid() {
		return errors.New("msgp: type " + val.String() + " not supported")
	}

	switch val.Kind() {
	case reflect.Ptr:
		if val.IsNil() {
			return mw.WriteNil()
		}
		return mw.WriteIntf(val.Elem().Interface())
	case reflect.Slice:
		return mw.writeSlice(val)
	case reflect.Map:
		return mw.writeMap(val)
	}
	return &ErrUnsupportedType{T: val.Type()}
}

func (mw *Writer) writeMap(v reflect.Value) (err error) {
	if v.Type().Key().Kind() != reflect.String {
		return errors.New("msgp: map keys must be strings")
	}
	ks := v.MapKeys()
	err = mw.WriteMapHeader(uint32(len(ks)))
	if err != nil {
		return
	}
	for _, key := range ks {
		val := v.MapIndex(key)
		err = mw.WriteString(key.String())
		if err != nil {
			return
		}
		err = mw.WriteIntf(val.Interface())
		if err != nil {
			return
		}
	}
	return
}

func (mw *Writer) writeSlice(v reflect.Value) (err error) {
	// is []byte
	if v.Type().ConvertibleTo(btsType) {
		return mw.WriteBytes(v.Bytes())
	}

	sz := uint32(v.Len())
	err = mw.WriteArrayHeader(sz)
	if err != nil {
		return
	}
	for i := uint32(0); i < sz; i++ {
		err = mw.WriteIntf(v.Index(int(i)).Interface())
		if err != nil {
			return
		}
	}
	return
}

// is the reflect.Kind encodable?
func isSupported(k reflect.Kind) bool {
	switch k {
	case reflect.Func, reflect.Chan, reflect.Invalid, reflect.UnsafePointer:
		return false
	default:
		return true
	}
}

// GuessSize guesses the size of the underlying
// value of 'i'. If the underlying value is not
// a simple builtin (or []byte), GuessSize defaults
// to 512.
func GuessSize(i interface{}) int {
	if i == nil {
		return NilSize
	}

	switch i := i.(type) {
	case Sizer:
		return i.Msgsize()
	case Extension:
		return ExtensionPrefixSize + i.Len()
	case float64:
		return Float64Size
	case float32:
		return Float32Size
	case uint8, uint16, uint32, uint64, uint:
		return UintSize
	case int8, int16, int32, int64, int:
		return IntSize
	case []byte:
		return BytesPrefixSize + len(i)
	case string:
		return StringPrefixSize + len(i)
	case complex64:
		return Complex64Size
	case complex128:
		return Complex128Size
	case bool:
		return BoolSize
	case map[string]interface{}:
		s := MapHeaderSize
		for key, val := range i {
			s += StringPrefixSize + len(key) + GuessSize(val)
		}
		return s
	case map[string]string:
		s := MapHeaderSize
		for key, val := range i {
			s += 2*StringPrefixSize + len(key) + len(val)
		}
		return s
	default:
		return 512
	}
}
