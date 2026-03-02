package u32

// LeBytes returns a byte slice corresponding to the 4 bytes in the uint32 in little-endian byte order.
func LeBytes(v uint32) []byte {
	return []byte{
		byte(v),
		byte(v >> 8),
		byte(v >> 16),
		byte(v >> 24),
	}
}
