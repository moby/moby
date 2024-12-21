package compression

// Compression is the state represents if compressed or not.
type Compression int

const (
	None  Compression = 0 // None represents the uncompressed.
	Bzip2 Compression = 1 // Bzip2 is bzip2 compression algorithm.
	Gzip  Compression = 2 // Gzip is gzip compression algorithm.
	Xz    Compression = 3 // Xz is xz compression algorithm.
	Zstd  Compression = 4 // Zstd is zstd compression algorithm.
)

// Extension returns the extension of a file that uses the specified compression algorithm.
func (compression *Compression) Extension() string {
	switch *compression {
	case None:
		return "tar"
	case Bzip2:
		return "tar.bz2"
	case Gzip:
		return "tar.gz"
	case Xz:
		return "tar.xz"
	case Zstd:
		return "tar.zst"
	}
	return ""
}
