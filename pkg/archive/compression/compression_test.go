package compression

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"testing"
	"time"
)

var tmp string

func init() {
	tmp = "/tmp/"
	if runtime.GOOS == "windows" {
		tmp = os.Getenv("TEMP") + `\`
	}
}

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

func testDecompressStream(t *testing.T, ext, compressCommand string) io.Reader {
	cmd := exec.Command("sh", "-c",
		fmt.Sprintf("touch /tmp/archive && %s /tmp/archive", compressCommand))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to create an archive file for test : %s.", output)
	}
	filename := "archive." + ext
	archive, err := os.Open(tmp + filename)
	if err != nil {
		t.Fatalf("Failed to open file %s: %v", filename, err)
	}
	defer archive.Close()

	r, err := DecompressStream(archive)
	if err != nil {
		t.Fatalf("Failed to decompress %s: %v", filename, err)
	}
	if _, err = io.ReadAll(r); err != nil {
		t.Fatalf("Failed to read the decompressed stream: %v ", err)
	}
	if err = r.Close(); err != nil {
		t.Fatalf("Failed to close the decompressed stream: %v ", err)
	}

	return r
}

func TestDecompressStreamGzip(t *testing.T) {
	testDecompressStream(t, "gz", "gzip -f")
}

func TestDecompressStreamBzip2(t *testing.T) {
	testDecompressStream(t, "bz2", "bzip2 -f")
}

func TestDecompressStreamXz(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Xz not present in msys2")
	}
	testDecompressStream(t, "xz", "xz -f")
}

func TestDecompressStreamZstd(t *testing.T) {
	if _, err := exec.LookPath("zstd"); err != nil {
		t.Skip("zstd not installed")
	}
	testDecompressStream(t, "zst", "zstd -f")
}

func TestCompressStreamXzUnsupported(t *testing.T) {
	dest, err := os.Create(tmp + "dest")
	if err != nil {
		t.Fatalf("Fail to create the destination file")
	}
	defer dest.Close()

	_, err = CompressStream(dest, Xz)
	if err == nil {
		t.Fatalf("Should fail as xz is unsupported for compression format.")
	}
}

func TestCompressStreamBzip2Unsupported(t *testing.T) {
	dest, err := os.Create(tmp + "dest")
	if err != nil {
		t.Fatalf("Fail to create the destination file")
	}
	defer dest.Close()

	_, err = CompressStream(dest, Bzip2)
	if err == nil {
		t.Fatalf("Should fail as bzip2 is unsupported for compression format.")
	}
}

func TestCompressStreamInvalid(t *testing.T) {
	dest, err := os.Create(tmp + "dest")
	if err != nil {
		t.Fatalf("Fail to create the destination file")
	}
	defer dest.Close()

	_, err = CompressStream(dest, -1)
	if err == nil {
		t.Fatalf("Should fail as xz is unsupported for compression format.")
	}
}

func TestCmdStreamLargeStderr(t *testing.T) {
	cmd := exec.Command("sh", "-c", "dd if=/dev/zero bs=1k count=1000 of=/dev/stderr; echo hello")
	out, err := cmdStream(cmd, nil)
	if err != nil {
		t.Fatalf("Failed to start command: %s", err)
	}
	errCh := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, out)
		errCh <- err
	}()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Command should not have failed (err=%.100s...)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Command did not complete in 5 seconds; probable deadlock")
	}
}

func TestCmdStreamBad(t *testing.T) {
	// TODO Windows: Figure out why this is failing in CI but not locally
	if runtime.GOOS == "windows" {
		t.Skip("Failing on Windows CI machines")
	}
	badCmd := exec.Command("sh", "-c", "echo hello; echo >&2 error couldn\\'t reverse the phase pulser; exit 1")
	out, err := cmdStream(badCmd, nil)
	if err != nil {
		t.Fatalf("Failed to start command: %s", err)
	}
	if output, err := io.ReadAll(out); err == nil {
		t.Fatalf("Command should have failed")
	} else if err.Error() != "exit status 1: error couldn't reverse the phase pulser\n" {
		t.Fatalf("Wrong error value (%s)", err)
	} else if s := string(output); s != "hello\n" {
		t.Fatalf("Command output should be '%s', not '%s'", "hello\\n", output)
	}
}

func TestCmdStreamGood(t *testing.T) {
	cmd := exec.Command("sh", "-c", "echo hello; exit 0")
	out, err := cmdStream(cmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if output, err := io.ReadAll(out); err != nil {
		t.Fatalf("Command should not have failed (err=%s)", err)
	} else if s := string(output); s != "hello\n" {
		t.Fatalf("Command output should be '%s', not '%s'", "hello\\n", output)
	}
}

func TestDisablePigz(t *testing.T) {
	_, err := exec.LookPath("unpigz")
	if err != nil {
		t.Log("Test will not check full path when Pigz not installed")
	}

	t.Setenv("MOBY_DISABLE_PIGZ", "true")

	r := testDecompressStream(t, "gz", "gzip -f")

	// wrapped in closer to cancel contex and release buffer to pool
	wrapper := r.(*readCloserWrapper)

	expected := reflect.TypeOf(&gzip.Reader{})
	actual := reflect.TypeOf(wrapper.Reader)
	if actual != expected {
		t.Fatalf("Expected: %v, Actual: %v", expected, actual)
	}
}

func TestPigz(t *testing.T) {
	r := testDecompressStream(t, "gz", "gzip -f")
	// wrapper for buffered reader and context cancel
	wrapper := r.(*readCloserWrapper)

	_, err := exec.LookPath("unpigz")
	if err == nil {
		t.Log("Tested whether Pigz is used, as it installed")
		// For the command wait wrapper
		cmdWaitCloserWrapper := wrapper.Reader.(*readCloserWrapper)
		expected := reflect.TypeOf(&io.PipeReader{})
		actual := reflect.TypeOf(cmdWaitCloserWrapper.Reader)
		if actual != expected {
			t.Fatalf("Expected: %v, Actual: %v", expected, actual)
		}
	} else {
		t.Log("Tested whether Pigz is not used, as it not installed")
		expected := reflect.TypeOf(&gzip.Reader{})
		actual := reflect.TypeOf(wrapper.Reader)
		if actual != expected {
			t.Fatalf("Expected: %v, Actual: %v", expected, actual)
		}
	}
}
