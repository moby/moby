package iso9660wrap

import (
	"time"
)

func WriteDirectoryRecord(w *SectorWriter, identifier string, firstSectorNum uint32) uint32 {
	if len(identifier) > 30 {
		Panicf("directory identifier length %d is out of bounds", len(identifier))
	}
	recordLength := 33 + len(identifier)

	w.WriteByte(byte(recordLength))
	w.WriteByte(0) // number of sectors in extended attribute record
	w.WriteBothEndianDWord(firstSectorNum)
	w.WriteBothEndianDWord(SectorSize) // directory length
	writeDirectoryRecordtimestamp(w, time.Now())
	w.WriteByte(byte(3))     // bitfield; directory
	w.WriteByte(byte(0))     // file unit size for an interleaved file
	w.WriteByte(byte(0))     // interleave gap size for an interleaved file
	w.WriteBothEndianWord(1) // volume sequence number
	w.WriteByte(byte(len(identifier)))
	w.WriteString(identifier)
	// optional padding to even length
	if recordLength%2 == 1 {
		recordLength++
		w.WriteByte(0)
	}
	return uint32(recordLength)
}

func WriteFileRecordHeader(w *SectorWriter, identifier string, firstSectorNum uint32, fileSize uint32) uint32 {
	if len(identifier) > 30 {
		Panicf("directory identifier length %d is out of bounds", len(identifier))
	}
	recordLength := 33 + len(identifier)

	w.WriteByte(byte(recordLength))
	w.WriteByte(0)                         // number of sectors in extended attribute record
	w.WriteBothEndianDWord(firstSectorNum) // first sector
	w.WriteBothEndianDWord(fileSize)
	writeDirectoryRecordtimestamp(w, time.Now())
	w.WriteByte(byte(0))     // bitfield; normal file
	w.WriteByte(byte(0))     // file unit size for an interleaved file
	w.WriteByte(byte(0))     // interleave gap size for an interleaved file
	w.WriteBothEndianWord(1) // volume sequence number
	w.WriteByte(byte(len(identifier)))
	w.WriteString(identifier)
	// optional padding to even length
	if recordLength%2 == 1 {
		recordLength++
		w.WriteByte(0)
	}
	return uint32(recordLength)
}

func writeDirectoryRecordtimestamp(w *SectorWriter, t time.Time) {
	t = t.UTC()
	w.WriteByte(byte(t.Year() - 1900))
	w.WriteByte(byte(t.Month()))
	w.WriteByte(byte(t.Day()))
	w.WriteByte(byte(t.Hour()))
	w.WriteByte(byte(t.Minute()))
	w.WriteByte(byte(t.Second()))
	w.WriteByte(0) // UTC offset
}
