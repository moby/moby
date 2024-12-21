package compression

import "testing"

func TestExtensionInvalid(t *testing.T) {
	compression := Compression(-1)
	output := compression.Extension()
	if output != "" {
		t.Fatalf("The extension of an invalid compression should be an empty string.")
	}
}

func TestExtensionUncompressed(t *testing.T) {
	compression := None
	output := compression.Extension()
	if output != "tar" {
		t.Fatalf("The extension of an uncompressed archive should be 'tar'.")
	}
}

func TestExtensionBzip2(t *testing.T) {
	compression := Bzip2
	output := compression.Extension()
	if output != "tar.bz2" {
		t.Fatalf("The extension of a bzip2 archive should be 'tar.bz2'")
	}
}

func TestExtensionGzip(t *testing.T) {
	compression := Gzip
	output := compression.Extension()
	if output != "tar.gz" {
		t.Fatalf("The extension of a gzip archive should be 'tar.gz'")
	}
}

func TestExtensionXz(t *testing.T) {
	compression := Xz
	output := compression.Extension()
	if output != "tar.xz" {
		t.Fatalf("The extension of a xz archive should be 'tar.xz'")
	}
}

func TestExtensionZstd(t *testing.T) {
	compression := Zstd
	output := compression.Extension()
	if output != "tar.zst" {
		t.Fatalf("The extension of a zstd archive should be 'tar.zst'")
	}
}

func TestDetectCompressionZstd(t *testing.T) {
	// test zstd compression without skippable frames.
	compressedData := []byte{
		0x28, 0xb5, 0x2f, 0xfd, // magic number of Zstandard frame: 0xFD2FB528
		0x04, 0x00, 0x31, 0x00, 0x00, // frame header
		0x64, 0x6f, 0x63, 0x6b, 0x65, 0x72, // data block "docker"
		0x16, 0x0e, 0x21, 0xc3, // content checksum
	}
	compression := Detect(compressedData)
	if compression != Zstd {
		t.Fatal("Unexpected compression")
	}
	// test zstd compression with skippable frames.
	hex := []byte{
		0x50, 0x2a, 0x4d, 0x18, // magic number of skippable frame: 0x184D2A50 to 0x184D2A5F
		0x04, 0x00, 0x00, 0x00, // frame size
		0x5d, 0x00, 0x00, 0x00, // user data
		0x28, 0xb5, 0x2f, 0xfd, // magic number of Zstandard frame: 0xFD2FB528
		0x04, 0x00, 0x31, 0x00, 0x00, // frame header
		0x64, 0x6f, 0x63, 0x6b, 0x65, 0x72, // data block "docker"
		0x16, 0x0e, 0x21, 0xc3, // content checksum
	}
	compression = Detect(hex)
	if compression != Zstd {
		t.Fatal("Unexpected compression")
	}
}
