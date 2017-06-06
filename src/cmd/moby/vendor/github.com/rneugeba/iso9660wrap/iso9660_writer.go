package iso9660wrap

import (
	"encoding/binary"
	"io"
	"math"
	"strings"
	"time"
)

const SectorSize uint32 = 2048

type SectorWriter struct {
	w io.Writer
	p uint32
}

func (w *SectorWriter) Write(p []byte) uint32 {
	if len(p) >= math.MaxUint32 {
		Panicf("attempted write of length %d is out of sector bounds", len(p))
	}
	l := uint32(len(p))
	if l > w.RemainingSpace() {
		Panicf("attempted write of length %d at offset %d is out of sector bounds", w.p, len(p))
	}
	w.p += l
	_, err := w.w.Write(p)
	if err != nil {
		panic(err)
	}
	return l
}

func (w *SectorWriter) WriteUnspecifiedDateTime() uint32 {
	b := make([]byte, 17)
	for i := 0; i < 16; i++ {
		b[i] = '0'
	}
	b[16] = 0
	return w.Write(b)
}

func (w *SectorWriter) WriteDateTime(t time.Time) uint32 {
	f := t.UTC().Format("20060102150405")
	f += "00"   // 1/100
	f += "\x00" // UTC offset
	if len(f) != 17 {
		Panicf("date and time field %q is of unexpected length %d", f, len(f))
	}
	return w.WriteString(f)
}

func (w *SectorWriter) WriteString(str string) uint32 {
	return w.Write([]byte(str))
}

func (w *SectorWriter) WritePaddedString(str string, length uint32) uint32 {
	l := w.WriteString(str)
	if l > 32 {
		Panicf("padded string %q exceeds length %d", str, length)
	} else if l < 32 {
		w.WriteString(strings.Repeat(" ", int(32-l)))
	}
	return 32
}

func (w *SectorWriter) WriteByte(b byte) uint32 {
	return w.Write([]byte{b})
}

func (w *SectorWriter) WriteWord(bo binary.ByteOrder, word uint16) uint32 {
	b := make([]byte, 2)
	bo.PutUint16(b, word)
	return w.Write(b)
}

func (w *SectorWriter) WriteBothEndianWord(word uint16) uint32 {
	w.WriteWord(binary.LittleEndian, word)
	w.WriteWord(binary.BigEndian, word)
	return 4
}

func (w *SectorWriter) WriteDWord(bo binary.ByteOrder, dword uint32) uint32 {
	b := make([]byte, 4)
	bo.PutUint32(b, dword)
	return w.Write(b)
}

func (w *SectorWriter) WriteLittleEndianDWord(dword uint32) uint32 {
	return w.WriteDWord(binary.LittleEndian, dword)
}

func (w *SectorWriter) WriteBigEndianDWord(dword uint32) uint32 {
	return w.WriteDWord(binary.BigEndian, dword)
}

func (w *SectorWriter) WriteBothEndianDWord(dword uint32) uint32 {
	w.WriteLittleEndianDWord(dword)
	w.WriteBigEndianDWord(dword)
	return 8
}

func (w *SectorWriter) WriteZeros(c int) uint32 {
	return w.Write(make([]byte, c))
}

func (w *SectorWriter) PadWithZeros() uint32 {
	return w.Write(make([]byte, w.RemainingSpace()))
}

func (w *SectorWriter) RemainingSpace() uint32 {
	return SectorSize - w.p
}

func (w *SectorWriter) Reset() {
	w.p = 0
}

type ISO9660Writer struct {
	sw        *SectorWriter
	sectorNum uint32
}

func (w *ISO9660Writer) CurrentSector() uint32 {
	return uint32(w.sectorNum)
}

func (w *ISO9660Writer) NextSector() *SectorWriter {
	if w.sw.RemainingSpace() == SectorSize {
		Panicf("internal error: tried to leave sector %d empty", w.sectorNum)
	}
	w.sw.PadWithZeros()
	w.sw.Reset()
	w.sectorNum++
	return w.sw
}

func (w *ISO9660Writer) Finish() {
	if w.sw.RemainingSpace() != SectorSize {
		w.sw.PadWithZeros()
	}
	w.sw = nil
}

func NewISO9660Writer(w io.Writer) *ISO9660Writer {
	// start at the end of the last reserved sector
	return &ISO9660Writer{&SectorWriter{w, SectorSize}, 16 - 1}
}
