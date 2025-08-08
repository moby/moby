package msgp

import "encoding/binary"

/* ----------------------------------
	integer encoding utilities
	(inline-able)

	TODO(tinylib): there are faster,
	albeit non-portable solutions
	to the code below. implement
	byteswap?
   ---------------------------------- */

func putMint64(b []byte, i int64) {
	_ = b[8] // bounds check elimination

	b[0] = mint64
	b[1] = byte(i >> 56)
	b[2] = byte(i >> 48)
	b[3] = byte(i >> 40)
	b[4] = byte(i >> 32)
	b[5] = byte(i >> 24)
	b[6] = byte(i >> 16)
	b[7] = byte(i >> 8)
	b[8] = byte(i)
}

func getMint64(b []byte) int64 {
	_ = b[8] // bounds check elimination

	return (int64(b[1]) << 56) | (int64(b[2]) << 48) |
		(int64(b[3]) << 40) | (int64(b[4]) << 32) |
		(int64(b[5]) << 24) | (int64(b[6]) << 16) |
		(int64(b[7]) << 8) | (int64(b[8]))
}

func putMint32(b []byte, i int32) {
	_ = b[4] // bounds check elimination

	b[0] = mint32
	b[1] = byte(i >> 24)
	b[2] = byte(i >> 16)
	b[3] = byte(i >> 8)
	b[4] = byte(i)
}

func getMint32(b []byte) int32 {
	_ = b[4] // bounds check elimination

	return (int32(b[1]) << 24) | (int32(b[2]) << 16) | (int32(b[3]) << 8) | (int32(b[4]))
}

func putMint16(b []byte, i int16) {
	_ = b[2] // bounds check elimination

	b[0] = mint16
	b[1] = byte(i >> 8)
	b[2] = byte(i)
}

func getMint16(b []byte) (i int16) {
	_ = b[2] // bounds check elimination

	return (int16(b[1]) << 8) | int16(b[2])
}

func putMint8(b []byte, i int8) {
	_ = b[1] // bounds check elimination

	b[0] = mint8
	b[1] = byte(i)
}

func getMint8(b []byte) (i int8) {
	return int8(b[1])
}

func putMuint64(b []byte, u uint64) {
	_ = b[8] // bounds check elimination

	b[0] = muint64
	b[1] = byte(u >> 56)
	b[2] = byte(u >> 48)
	b[3] = byte(u >> 40)
	b[4] = byte(u >> 32)
	b[5] = byte(u >> 24)
	b[6] = byte(u >> 16)
	b[7] = byte(u >> 8)
	b[8] = byte(u)
}

func getMuint64(b []byte) uint64 {
	_ = b[8] // bounds check elimination

	return (uint64(b[1]) << 56) | (uint64(b[2]) << 48) |
		(uint64(b[3]) << 40) | (uint64(b[4]) << 32) |
		(uint64(b[5]) << 24) | (uint64(b[6]) << 16) |
		(uint64(b[7]) << 8) | (uint64(b[8]))
}

func putMuint32(b []byte, u uint32) {
	_ = b[4] // bounds check elimination

	b[0] = muint32
	b[1] = byte(u >> 24)
	b[2] = byte(u >> 16)
	b[3] = byte(u >> 8)
	b[4] = byte(u)
}

func getMuint32(b []byte) uint32 {
	_ = b[4] // bounds check elimination

	return (uint32(b[1]) << 24) | (uint32(b[2]) << 16) | (uint32(b[3]) << 8) | (uint32(b[4]))
}

func putMuint16(b []byte, u uint16) {
	_ = b[2] // bounds check elimination

	b[0] = muint16
	b[1] = byte(u >> 8)
	b[2] = byte(u)
}

func getMuint16(b []byte) uint16 {
	_ = b[2] // bounds check elimination

	return (uint16(b[1]) << 8) | uint16(b[2])
}

func putMuint8(b []byte, u uint8) {
	_ = b[1] // bounds check elimination

	b[0] = muint8
	b[1] = byte(u)
}

func getMuint8(b []byte) uint8 {
	return uint8(b[1])
}

func getUnix(b []byte) (sec int64, nsec int32) {
	sec = int64(binary.BigEndian.Uint64(b))
	nsec = int32(binary.BigEndian.Uint32(b[8:]))

	return
}

func putUnix(b []byte, sec int64, nsec int32) {
	binary.BigEndian.PutUint64(b, uint64(sec))
	binary.BigEndian.PutUint32(b[8:], uint32(nsec))
}

/* -----------------------------
		prefix utilities
   ----------------------------- */

// write prefix and uint8
func prefixu8(b []byte, pre byte, sz uint8) {
	_ = b[1] // bounds check elimination

	b[0] = pre
	b[1] = byte(sz)
}

// write prefix and big-endian uint16
func prefixu16(b []byte, pre byte, sz uint16) {
	_ = b[2] // bounds check elimination

	b[0] = pre
	b[1] = byte(sz >> 8)
	b[2] = byte(sz)
}

// write prefix and big-endian uint32
func prefixu32(b []byte, pre byte, sz uint32) {
	_ = b[4] // bounds check elimination

	b[0] = pre
	b[1] = byte(sz >> 24)
	b[2] = byte(sz >> 16)
	b[3] = byte(sz >> 8)
	b[4] = byte(sz)
}

func prefixu64(b []byte, pre byte, sz uint64) {
	_ = b[8] // bounds check elimination

	b[0] = pre
	b[1] = byte(sz >> 56)
	b[2] = byte(sz >> 48)
	b[3] = byte(sz >> 40)
	b[4] = byte(sz >> 32)
	b[5] = byte(sz >> 24)
	b[6] = byte(sz >> 16)
	b[7] = byte(sz >> 8)
	b[8] = byte(sz)
}
