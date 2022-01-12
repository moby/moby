package archive // import "github.com/docker/docker/pkg/archive"

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/ioutils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

var tmp string

func init() {
	tmp = "/tmp/"
	if runtime.GOOS == "windows" {
		tmp = os.Getenv("TEMP") + `\`
	}
}

var defaultArchiver = NewDefaultArchiver()

func defaultTarUntar(src, dst string) error {
	return defaultArchiver.TarUntar(src, dst)
}

func defaultUntarPath(src, dst string) error {
	return defaultArchiver.UntarPath(src, dst)
}

func defaultCopyFileWithTar(src, dst string) (err error) {
	return defaultArchiver.CopyFileWithTar(src, dst)
}

func defaultCopyWithTar(src, dst string) error {
	return defaultArchiver.CopyWithTar(src, dst)
}

func TestIsArchivePathDir(t *testing.T) {
	cmd := exec.Command("sh", "-c", "mkdir -p /tmp/archivedir")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Fail to create an archive file for test : %s.", output)
	}
	if IsArchivePath(tmp + "archivedir") {
		t.Fatalf("Incorrectly recognised directory as an archive")
	}
}

func TestIsArchivePathInvalidFile(t *testing.T) {
	cmd := exec.Command("sh", "-c", "dd if=/dev/zero bs=1024 count=1 of=/tmp/archive && gzip --stdout /tmp/archive > /tmp/archive.gz")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Fail to create an archive file for test : %s.", output)
	}
	if IsArchivePath(tmp + "archive") {
		t.Fatalf("Incorrectly recognised invalid tar path as archive")
	}
	if IsArchivePath(tmp + "archive.gz") {
		t.Fatalf("Incorrectly recognised invalid compressed tar path as archive")
	}
}

func TestIsArchivePathTar(t *testing.T) {
	whichTar := "tar"
	cmdStr := fmt.Sprintf("touch /tmp/archivedata && %s -cf /tmp/archive /tmp/archivedata && gzip --stdout /tmp/archive > /tmp/archive.gz", whichTar)
	cmd := exec.Command("sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Fail to create an archive file for test : %s.", output)
	}
	if !IsArchivePath(tmp + "/archive") {
		t.Fatalf("Did not recognise valid tar path as archive")
	}
	if !IsArchivePath(tmp + "archive.gz") {
		t.Fatalf("Did not recognise valid compressed tar path as archive")
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

func TestExtensionInvalid(t *testing.T) {
	compression := Compression(-1)
	output := compression.Extension()
	if output != "" {
		t.Fatalf("The extension of an invalid compression should be an empty string.")
	}
}

func TestExtensionUncompressed(t *testing.T) {
	compression := Uncompressed
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

func TestUntarPathWithInvalidDest(t *testing.T) {
	tempFolder, err := os.MkdirTemp("", "docker-archive-test")
	assert.NilError(t, err)
	defer os.RemoveAll(tempFolder)
	invalidDestFolder := filepath.Join(tempFolder, "invalidDest")
	// Create a src file
	srcFile := filepath.Join(tempFolder, "src")
	tarFile := filepath.Join(tempFolder, "src.tar")
	os.Create(srcFile)
	os.Create(invalidDestFolder) // being a file (not dir) should cause an error

	// Translate back to Unix semantics as next exec.Command is run under sh
	srcFileU := srcFile
	tarFileU := tarFile
	if runtime.GOOS == "windows" {
		tarFileU = "/tmp/" + filepath.Base(filepath.Dir(tarFile)) + "/src.tar"
		srcFileU = "/tmp/" + filepath.Base(filepath.Dir(srcFile)) + "/src"
	}

	cmd := exec.Command("sh", "-c", "tar cf "+tarFileU+" "+srcFileU)
	_, err = cmd.CombinedOutput()
	assert.NilError(t, err)

	err = defaultUntarPath(tarFile, invalidDestFolder)
	if err == nil {
		t.Fatalf("UntarPath with invalid destination path should throw an error.")
	}
}

func TestUntarPathWithInvalidSrc(t *testing.T) {
	dest, err := os.MkdirTemp("", "docker-archive-test")
	if err != nil {
		t.Fatalf("Fail to create the destination file")
	}
	defer os.RemoveAll(dest)
	err = defaultUntarPath("/invalid/path", dest)
	if err == nil {
		t.Fatalf("UntarPath with invalid src path should throw an error.")
	}
}

func TestUntarPath(t *testing.T) {
	skip.If(t, runtime.GOOS != "windows" && os.Getuid() != 0, "skipping test that requires root")
	tmpFolder, err := os.MkdirTemp("", "docker-archive-test")
	assert.NilError(t, err)
	defer os.RemoveAll(tmpFolder)
	srcFile := filepath.Join(tmpFolder, "src")
	tarFile := filepath.Join(tmpFolder, "src.tar")
	os.Create(filepath.Join(tmpFolder, "src"))

	destFolder := filepath.Join(tmpFolder, "dest")
	err = os.MkdirAll(destFolder, 0740)
	if err != nil {
		t.Fatalf("Fail to create the destination file")
	}

	// Translate back to Unix semantics as next exec.Command is run under sh
	srcFileU := srcFile
	tarFileU := tarFile
	if runtime.GOOS == "windows" {
		tarFileU = "/tmp/" + filepath.Base(filepath.Dir(tarFile)) + "/src.tar"
		srcFileU = "/tmp/" + filepath.Base(filepath.Dir(srcFile)) + "/src"
	}
	cmd := exec.Command("sh", "-c", "tar cf "+tarFileU+" "+srcFileU)
	_, err = cmd.CombinedOutput()
	assert.NilError(t, err)

	err = defaultUntarPath(tarFile, destFolder)
	if err != nil {
		t.Fatalf("UntarPath shouldn't throw an error, %s.", err)
	}
	expectedFile := filepath.Join(destFolder, srcFileU)
	_, err = os.Stat(expectedFile)
	if err != nil {
		t.Fatalf("Destination folder should contain the source file but did not.")
	}
}

// Do the same test as above but with the destination as file, it should fail
func TestUntarPathWithDestinationFile(t *testing.T) {
	tmpFolder, err := os.MkdirTemp("", "docker-archive-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpFolder)
	srcFile := filepath.Join(tmpFolder, "src")
	tarFile := filepath.Join(tmpFolder, "src.tar")
	os.Create(filepath.Join(tmpFolder, "src"))

	// Translate back to Unix semantics as next exec.Command is run under sh
	srcFileU := srcFile
	tarFileU := tarFile
	if runtime.GOOS == "windows" {
		tarFileU = "/tmp/" + filepath.Base(filepath.Dir(tarFile)) + "/src.tar"
		srcFileU = "/tmp/" + filepath.Base(filepath.Dir(srcFile)) + "/src"
	}
	cmd := exec.Command("sh", "-c", "tar cf "+tarFileU+" "+srcFileU)
	_, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	destFile := filepath.Join(tmpFolder, "dest")
	_, err = os.Create(destFile)
	if err != nil {
		t.Fatalf("Fail to create the destination file")
	}
	err = defaultUntarPath(tarFile, destFile)
	if err == nil {
		t.Fatalf("UntarPath should throw an error if the destination if a file")
	}
}

// Do the same test as above but with the destination folder already exists
// and the destination file is a directory
// It's working, see https://github.com/docker/docker/issues/10040
func TestUntarPathWithDestinationSrcFileAsFolder(t *testing.T) {
	tmpFolder, err := os.MkdirTemp("", "docker-archive-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpFolder)
	srcFile := filepath.Join(tmpFolder, "src")
	tarFile := filepath.Join(tmpFolder, "src.tar")
	os.Create(srcFile)

	// Translate back to Unix semantics as next exec.Command is run under sh
	srcFileU := srcFile
	tarFileU := tarFile
	if runtime.GOOS == "windows" {
		tarFileU = "/tmp/" + filepath.Base(filepath.Dir(tarFile)) + "/src.tar"
		srcFileU = "/tmp/" + filepath.Base(filepath.Dir(srcFile)) + "/src"
	}

	cmd := exec.Command("sh", "-c", "tar cf "+tarFileU+" "+srcFileU)
	_, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	destFolder := filepath.Join(tmpFolder, "dest")
	err = os.MkdirAll(destFolder, 0740)
	if err != nil {
		t.Fatalf("Fail to create the destination folder")
	}
	// Let's create a folder that will has the same path as the extracted file (from tar)
	destSrcFileAsFolder := filepath.Join(destFolder, srcFileU)
	err = os.MkdirAll(destSrcFileAsFolder, 0740)
	if err != nil {
		t.Fatal(err)
	}
	err = defaultUntarPath(tarFile, destFolder)
	if err != nil {
		t.Fatalf("UntarPath should throw not throw an error if the extracted file already exists and is a folder")
	}
}

func TestCopyWithTarInvalidSrc(t *testing.T) {
	tempFolder, err := os.MkdirTemp("", "docker-archive-test")
	if err != nil {
		t.Fatal(nil)
	}
	destFolder := filepath.Join(tempFolder, "dest")
	invalidSrc := filepath.Join(tempFolder, "doesnotexists")
	err = os.MkdirAll(destFolder, 0740)
	if err != nil {
		t.Fatal(err)
	}
	err = defaultCopyWithTar(invalidSrc, destFolder)
	if err == nil {
		t.Fatalf("archiver.CopyWithTar with invalid src path should throw an error.")
	}
}

func TestCopyWithTarInexistentDestWillCreateIt(t *testing.T) {
	skip.If(t, runtime.GOOS != "windows" && os.Getuid() != 0, "skipping test that requires root")
	tempFolder, err := os.MkdirTemp("", "docker-archive-test")
	if err != nil {
		t.Fatal(nil)
	}
	srcFolder := filepath.Join(tempFolder, "src")
	inexistentDestFolder := filepath.Join(tempFolder, "doesnotexists")
	err = os.MkdirAll(srcFolder, 0740)
	if err != nil {
		t.Fatal(err)
	}
	err = defaultCopyWithTar(srcFolder, inexistentDestFolder)
	if err != nil {
		t.Fatalf("CopyWithTar with an inexistent folder shouldn't fail.")
	}
	_, err = os.Stat(inexistentDestFolder)
	if err != nil {
		t.Fatalf("CopyWithTar with an inexistent folder should create it.")
	}
}

// Test CopyWithTar with a file as src
func TestCopyWithTarSrcFile(t *testing.T) {
	folder, err := os.MkdirTemp("", "docker-archive-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(folder)
	dest := filepath.Join(folder, "dest")
	srcFolder := filepath.Join(folder, "src")
	src := filepath.Join(folder, filepath.Join("src", "src"))
	err = os.MkdirAll(srcFolder, 0740)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(dest, 0740)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(src, []byte("content"), 0777)
	err = defaultCopyWithTar(src, dest)
	if err != nil {
		t.Fatalf("archiver.CopyWithTar shouldn't throw an error, %s.", err)
	}
	_, err = os.Stat(dest)
	// FIXME Check the content
	if err != nil {
		t.Fatalf("Destination file should be the same as the source.")
	}
}

// Test CopyWithTar with a folder as src
func TestCopyWithTarSrcFolder(t *testing.T) {
	folder, err := os.MkdirTemp("", "docker-archive-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(folder)
	dest := filepath.Join(folder, "dest")
	src := filepath.Join(folder, filepath.Join("src", "folder"))
	err = os.MkdirAll(src, 0740)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(dest, 0740)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(src, "file"), []byte("content"), 0777)
	err = defaultCopyWithTar(src, dest)
	if err != nil {
		t.Fatalf("archiver.CopyWithTar shouldn't throw an error, %s.", err)
	}
	_, err = os.Stat(dest)
	// FIXME Check the content (the file inside)
	if err != nil {
		t.Fatalf("Destination folder should contain the source file but did not.")
	}
}

func TestCopyFileWithTarInvalidSrc(t *testing.T) {
	tempFolder, err := os.MkdirTemp("", "docker-archive-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempFolder)
	destFolder := filepath.Join(tempFolder, "dest")
	err = os.MkdirAll(destFolder, 0740)
	if err != nil {
		t.Fatal(err)
	}
	invalidFile := filepath.Join(tempFolder, "doesnotexists")
	err = defaultCopyFileWithTar(invalidFile, destFolder)
	if err == nil {
		t.Fatalf("archiver.CopyWithTar with invalid src path should throw an error.")
	}
}

func TestCopyFileWithTarInexistentDestWillCreateIt(t *testing.T) {
	tempFolder, err := os.MkdirTemp("", "docker-archive-test")
	if err != nil {
		t.Fatal(nil)
	}
	defer os.RemoveAll(tempFolder)
	srcFile := filepath.Join(tempFolder, "src")
	inexistentDestFolder := filepath.Join(tempFolder, "doesnotexists")
	_, err = os.Create(srcFile)
	if err != nil {
		t.Fatal(err)
	}
	err = defaultCopyFileWithTar(srcFile, inexistentDestFolder)
	if err != nil {
		t.Fatalf("CopyWithTar with an inexistent folder shouldn't fail.")
	}
	_, err = os.Stat(inexistentDestFolder)
	if err != nil {
		t.Fatalf("CopyWithTar with an inexistent folder should create it.")
	}
	// FIXME Test the src file and content
}

func TestCopyFileWithTarSrcFolder(t *testing.T) {
	folder, err := os.MkdirTemp("", "docker-archive-copyfilewithtar-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(folder)
	dest := filepath.Join(folder, "dest")
	src := filepath.Join(folder, "srcfolder")
	err = os.MkdirAll(src, 0740)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(dest, 0740)
	if err != nil {
		t.Fatal(err)
	}
	err = defaultCopyFileWithTar(src, dest)
	if err == nil {
		t.Fatalf("CopyFileWithTar should throw an error with a folder.")
	}
}

func TestCopyFileWithTarSrcFile(t *testing.T) {
	folder, err := os.MkdirTemp("", "docker-archive-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(folder)
	dest := filepath.Join(folder, "dest")
	srcFolder := filepath.Join(folder, "src")
	src := filepath.Join(folder, filepath.Join("src", "src"))
	err = os.MkdirAll(srcFolder, 0740)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(dest, 0740)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(src, []byte("content"), 0777)
	err = defaultCopyWithTar(src, dest+"/")
	if err != nil {
		t.Fatalf("archiver.CopyFileWithTar shouldn't throw an error, %s.", err)
	}
	_, err = os.Stat(dest)
	if err != nil {
		t.Fatalf("Destination folder should contain the source file but did not.")
	}
}

func TestTarFiles(t *testing.T) {
	// try without hardlinks
	if err := checkNoChanges(1000, false); err != nil {
		t.Fatal(err)
	}
	// try with hardlinks
	if err := checkNoChanges(1000, true); err != nil {
		t.Fatal(err)
	}
}

func checkNoChanges(fileNum int, hardlinks bool) error {
	srcDir, err := os.MkdirTemp("", "docker-test-srcDir")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	destDir, err := os.MkdirTemp("", "docker-test-destDir")
	if err != nil {
		return err
	}
	defer os.RemoveAll(destDir)

	_, err = prepareUntarSourceDirectory(fileNum, srcDir, hardlinks)
	if err != nil {
		return err
	}

	err = defaultTarUntar(srcDir, destDir)
	if err != nil {
		return err
	}

	changes, err := ChangesDirs(destDir, srcDir)
	if err != nil {
		return err
	}
	if len(changes) > 0 {
		return fmt.Errorf("with %d files and %v hardlinks: expected 0 changes, got %d", fileNum, hardlinks, len(changes))
	}
	return nil
}

func tarUntar(t *testing.T, origin string, options *TarOptions) ([]Change, error) {
	archive, err := TarWithOptions(origin, options)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()

	buf := make([]byte, 10)
	if _, err := archive.Read(buf); err != nil {
		return nil, err
	}
	wrap := io.MultiReader(bytes.NewReader(buf), archive)

	detectedCompression := DetectCompression(buf)
	compression := options.Compression
	if detectedCompression.Extension() != compression.Extension() {
		return nil, fmt.Errorf("Wrong compression detected. Actual compression: %s, found %s", compression.Extension(), detectedCompression.Extension())
	}

	tmp, err := os.MkdirTemp("", "docker-test-untar")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := Untar(wrap, tmp, nil); err != nil {
		return nil, err
	}
	if _, err := os.Stat(tmp); err != nil {
		return nil, err
	}

	return ChangesDirs(origin, tmp)
}

func TestDetectCompressionZstd(t *testing.T) {
	// test zstd compression without skippable frames.
	compressedData := []byte{
		0x28, 0xb5, 0x2f, 0xfd, // magic number of Zstandard frame: 0xFD2FB528
		0x04, 0x00, 0x31, 0x00, 0x00, // frame header
		0x64, 0x6f, 0x63, 0x6b, 0x65, 0x72, // data block "docker"
		0x16, 0x0e, 0x21, 0xc3, // content checksum
	}
	compression := DetectCompression(compressedData)
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
	compression = DetectCompression(hex)
	if compression != Zstd {
		t.Fatal("Unexpected compression")
	}
}

func TestTarUntar(t *testing.T) {
	origin, err := os.MkdirTemp("", "docker-test-untar-origin")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(origin)
	if err := os.WriteFile(filepath.Join(origin, "1"), []byte("hello world"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(origin, "2"), []byte("welcome!"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(origin, "3"), []byte("will be ignored"), 0700); err != nil {
		t.Fatal(err)
	}

	for _, c := range []Compression{
		Uncompressed,
		Gzip,
	} {
		changes, err := tarUntar(t, origin, &TarOptions{
			Compression:     c,
			ExcludePatterns: []string{"3"},
		})

		if err != nil {
			t.Fatalf("Error tar/untar for compression %s: %s", c.Extension(), err)
		}

		if len(changes) != 1 || changes[0].Path != string(filepath.Separator)+"3" {
			t.Fatalf("Unexpected differences after tarUntar: %v", changes)
		}
	}
}

func TestTarWithOptionsChownOptsAlwaysOverridesIdPair(t *testing.T) {
	origin, err := os.MkdirTemp("", "docker-test-tar-chown-opt")
	assert.NilError(t, err)

	defer os.RemoveAll(origin)
	filePath := filepath.Join(origin, "1")
	err = os.WriteFile(filePath, []byte("hello world"), 0700)
	assert.NilError(t, err)

	idMaps := []idtools.IDMap{
		0: {
			ContainerID: 0,
			HostID:      0,
			Size:        65536,
		},
		1: {
			ContainerID: 0,
			HostID:      100000,
			Size:        65536,
		},
	}

	cases := []struct {
		opts        *TarOptions
		expectedUID int
		expectedGID int
	}{
		{&TarOptions{ChownOpts: &idtools.Identity{UID: 1337, GID: 42}}, 1337, 42},
		{&TarOptions{ChownOpts: &idtools.Identity{UID: 100001, GID: 100001}, UIDMaps: idMaps, GIDMaps: idMaps}, 100001, 100001},
		{&TarOptions{ChownOpts: &idtools.Identity{UID: 0, GID: 0}, NoLchown: false}, 0, 0},
		{&TarOptions{ChownOpts: &idtools.Identity{UID: 1, GID: 1}, NoLchown: true}, 1, 1},
		{&TarOptions{ChownOpts: &idtools.Identity{UID: 1000, GID: 1000}, NoLchown: true}, 1000, 1000},
	}
	for _, testCase := range cases {
		reader, err := TarWithOptions(filePath, testCase.opts)
		assert.NilError(t, err)
		tr := tar.NewReader(reader)
		defer reader.Close()
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				// end of tar archive
				break
			}
			assert.NilError(t, err)
			assert.Check(t, is.Equal(hdr.Uid, testCase.expectedUID), "Uid equals expected value")
			assert.Check(t, is.Equal(hdr.Gid, testCase.expectedGID), "Gid equals expected value")
		}
	}
}

func TestTarWithOptions(t *testing.T) {
	origin, err := os.MkdirTemp("", "docker-test-untar-origin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.MkdirTemp(origin, "folder"); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(origin)
	if err := os.WriteFile(filepath.Join(origin, "1"), []byte("hello world"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(origin, "2"), []byte("welcome!"), 0700); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		opts       *TarOptions
		numChanges int
	}{
		{&TarOptions{IncludeFiles: []string{"1"}}, 2},
		{&TarOptions{ExcludePatterns: []string{"2"}}, 1},
		{&TarOptions{ExcludePatterns: []string{"1", "folder*"}}, 2},
		{&TarOptions{IncludeFiles: []string{"1", "1"}}, 2},
		{&TarOptions{IncludeFiles: []string{"1"}, RebaseNames: map[string]string{"1": "test"}}, 4},
	}
	for _, testCase := range cases {
		changes, err := tarUntar(t, origin, testCase.opts)
		if err != nil {
			t.Fatalf("Error tar/untar when testing inclusion/exclusion: %s", err)
		}
		if len(changes) != testCase.numChanges {
			t.Errorf("Expected %d changes, got %d for %+v:",
				testCase.numChanges, len(changes), testCase.opts)
		}
	}
}

// Some tar archives such as http://haproxy.1wt.eu/download/1.5/src/devel/haproxy-1.5-dev21.tar.gz
// use PAX Global Extended Headers.
// Failing prevents the archives from being uncompressed during ADD
func TestTypeXGlobalHeaderDoesNotFail(t *testing.T) {
	hdr := tar.Header{Typeflag: tar.TypeXGlobalHeader}
	tmpDir, err := os.MkdirTemp("", "docker-test-archive-pax-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	err = createTarFile(filepath.Join(tmpDir, "pax_global_header"), tmpDir, &hdr, nil, true, nil, false)
	if err != nil {
		t.Fatal(err)
	}
}

// Some tar have both GNU specific (huge uid) and Ustar specific (long name) things.
// Not supposed to happen (should use PAX instead of Ustar for long name) but it does and it should still work.
func TestUntarUstarGnuConflict(t *testing.T) {
	f, err := os.Open("testdata/broken.tar")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	found := false
	tr := tar.NewReader(f)
	// Iterate through the files in the archive.
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name == "root/.cpanm/work/1395823785.24209/Plack-1.0030/blib/man3/Plack::Middleware::LighttpdScriptNameFix.3pm" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("%s not found in the archive", "root/.cpanm/work/1395823785.24209/Plack-1.0030/blib/man3/Plack::Middleware::LighttpdScriptNameFix.3pm")
	}
}

func prepareUntarSourceDirectory(numberOfFiles int, targetPath string, makeLinks bool) (int, error) {
	fileData := []byte("fooo")
	for n := 0; n < numberOfFiles; n++ {
		fileName := fmt.Sprintf("file-%d", n)
		if err := os.WriteFile(filepath.Join(targetPath, fileName), fileData, 0700); err != nil {
			return 0, err
		}
		if makeLinks {
			if err := os.Link(filepath.Join(targetPath, fileName), filepath.Join(targetPath, fileName+"-link")); err != nil {
				return 0, err
			}
		}
	}
	totalSize := numberOfFiles * len(fileData)
	return totalSize, nil
}

func BenchmarkTarUntar(b *testing.B) {
	origin, err := os.MkdirTemp("", "docker-test-untar-origin")
	if err != nil {
		b.Fatal(err)
	}
	tempDir, err := os.MkdirTemp("", "docker-test-untar-destination")
	if err != nil {
		b.Fatal(err)
	}
	target := filepath.Join(tempDir, "dest")
	n, err := prepareUntarSourceDirectory(100, origin, false)
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(origin)
	defer os.RemoveAll(tempDir)

	b.ResetTimer()
	b.SetBytes(int64(n))
	for n := 0; n < b.N; n++ {
		err := defaultTarUntar(origin, target)
		if err != nil {
			b.Fatal(err)
		}
		os.RemoveAll(target)
	}
}

func BenchmarkTarUntarWithLinks(b *testing.B) {
	origin, err := os.MkdirTemp("", "docker-test-untar-origin")
	if err != nil {
		b.Fatal(err)
	}
	tempDir, err := os.MkdirTemp("", "docker-test-untar-destination")
	if err != nil {
		b.Fatal(err)
	}
	target := filepath.Join(tempDir, "dest")
	n, err := prepareUntarSourceDirectory(100, origin, true)
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(origin)
	defer os.RemoveAll(tempDir)

	b.ResetTimer()
	b.SetBytes(int64(n))
	for n := 0; n < b.N; n++ {
		err := defaultTarUntar(origin, target)
		if err != nil {
			b.Fatal(err)
		}
		os.RemoveAll(target)
	}
}

func TestUntarInvalidFilenames(t *testing.T) {
	for i, headers := range [][]*tar.Header{
		{
			{
				Name:     "../victim/dotdot",
				Typeflag: tar.TypeReg,
				Mode:     0644,
			},
		},
		{
			{
				// Note the leading slash
				Name:     "/../victim/slash-dotdot",
				Typeflag: tar.TypeReg,
				Mode:     0644,
			},
		},
	} {
		if err := testBreakout("untar", "docker-TestUntarInvalidFilenames", headers); err != nil {
			t.Fatalf("i=%d. %v", i, err)
		}
	}
}

func TestUntarHardlinkToSymlink(t *testing.T) {
	skip.If(t, runtime.GOOS != "windows" && os.Getuid() != 0, "skipping test that requires root")
	for i, headers := range [][]*tar.Header{
		{
			{
				Name:     "symlink1",
				Typeflag: tar.TypeSymlink,
				Linkname: "regfile",
				Mode:     0644,
			},
			{
				Name:     "symlink2",
				Typeflag: tar.TypeLink,
				Linkname: "symlink1",
				Mode:     0644,
			},
			{
				Name:     "regfile",
				Typeflag: tar.TypeReg,
				Mode:     0644,
			},
		},
	} {
		if err := testBreakout("untar", "docker-TestUntarHardlinkToSymlink", headers); err != nil {
			t.Fatalf("i=%d. %v", i, err)
		}
	}
}

func TestUntarInvalidHardlink(t *testing.T) {
	for i, headers := range [][]*tar.Header{
		{ // try reading victim/hello (../)
			{
				Name:     "dotdot",
				Typeflag: tar.TypeLink,
				Linkname: "../victim/hello",
				Mode:     0644,
			},
		},
		{ // try reading victim/hello (/../)
			{
				Name:     "slash-dotdot",
				Typeflag: tar.TypeLink,
				// Note the leading slash
				Linkname: "/../victim/hello",
				Mode:     0644,
			},
		},
		{ // try writing victim/file
			{
				Name:     "loophole-victim",
				Typeflag: tar.TypeLink,
				Linkname: "../victim",
				Mode:     0755,
			},
			{
				Name:     "loophole-victim/file",
				Typeflag: tar.TypeReg,
				Mode:     0644,
			},
		},
		{ // try reading victim/hello (hardlink, symlink)
			{
				Name:     "loophole-victim",
				Typeflag: tar.TypeLink,
				Linkname: "../victim",
				Mode:     0755,
			},
			{
				Name:     "symlink",
				Typeflag: tar.TypeSymlink,
				Linkname: "loophole-victim/hello",
				Mode:     0644,
			},
		},
		{ // Try reading victim/hello (hardlink, hardlink)
			{
				Name:     "loophole-victim",
				Typeflag: tar.TypeLink,
				Linkname: "../victim",
				Mode:     0755,
			},
			{
				Name:     "hardlink",
				Typeflag: tar.TypeLink,
				Linkname: "loophole-victim/hello",
				Mode:     0644,
			},
		},
		{ // Try removing victim directory (hardlink)
			{
				Name:     "loophole-victim",
				Typeflag: tar.TypeLink,
				Linkname: "../victim",
				Mode:     0755,
			},
			{
				Name:     "loophole-victim",
				Typeflag: tar.TypeReg,
				Mode:     0644,
			},
		},
	} {
		if err := testBreakout("untar", "docker-TestUntarInvalidHardlink", headers); err != nil {
			t.Fatalf("i=%d. %v", i, err)
		}
	}
}

func TestUntarInvalidSymlink(t *testing.T) {
	for i, headers := range [][]*tar.Header{
		{ // try reading victim/hello (../)
			{
				Name:     "dotdot",
				Typeflag: tar.TypeSymlink,
				Linkname: "../victim/hello",
				Mode:     0644,
			},
		},
		{ // try reading victim/hello (/../)
			{
				Name:     "slash-dotdot",
				Typeflag: tar.TypeSymlink,
				// Note the leading slash
				Linkname: "/../victim/hello",
				Mode:     0644,
			},
		},
		{ // try writing victim/file
			{
				Name:     "loophole-victim",
				Typeflag: tar.TypeSymlink,
				Linkname: "../victim",
				Mode:     0755,
			},
			{
				Name:     "loophole-victim/file",
				Typeflag: tar.TypeReg,
				Mode:     0644,
			},
		},
		{ // try reading victim/hello (symlink, symlink)
			{
				Name:     "loophole-victim",
				Typeflag: tar.TypeSymlink,
				Linkname: "../victim",
				Mode:     0755,
			},
			{
				Name:     "symlink",
				Typeflag: tar.TypeSymlink,
				Linkname: "loophole-victim/hello",
				Mode:     0644,
			},
		},
		{ // try reading victim/hello (symlink, hardlink)
			{
				Name:     "loophole-victim",
				Typeflag: tar.TypeSymlink,
				Linkname: "../victim",
				Mode:     0755,
			},
			{
				Name:     "hardlink",
				Typeflag: tar.TypeLink,
				Linkname: "loophole-victim/hello",
				Mode:     0644,
			},
		},
		{ // try removing victim directory (symlink)
			{
				Name:     "loophole-victim",
				Typeflag: tar.TypeSymlink,
				Linkname: "../victim",
				Mode:     0755,
			},
			{
				Name:     "loophole-victim",
				Typeflag: tar.TypeReg,
				Mode:     0644,
			},
		},
		{ // try writing to victim/newdir/newfile with a symlink in the path
			{
				// this header needs to be before the next one, or else there is an error
				Name:     "dir/loophole",
				Typeflag: tar.TypeSymlink,
				Linkname: "../../victim",
				Mode:     0755,
			},
			{
				Name:     "dir/loophole/newdir/newfile",
				Typeflag: tar.TypeReg,
				Mode:     0644,
			},
		},
	} {
		if err := testBreakout("untar", "docker-TestUntarInvalidSymlink", headers); err != nil {
			t.Fatalf("i=%d. %v", i, err)
		}
	}
}

func TestTempArchiveCloseMultipleTimes(t *testing.T) {
	reader := io.NopCloser(strings.NewReader("hello"))
	tempArchive, err := NewTempArchive(reader, "")
	assert.NilError(t, err)
	buf := make([]byte, 10)
	n, err := tempArchive.Read(buf)
	assert.NilError(t, err)
	if n != 5 {
		t.Fatalf("Expected to read 5 bytes. Read %d instead", n)
	}
	for i := 0; i < 3; i++ {
		if err = tempArchive.Close(); err != nil {
			t.Fatalf("i=%d. Unexpected error closing temp archive: %v", i, err)
		}
	}
}

// TestXGlobalNoParent is a regression test to check parent directories are not crated for PAX headers
func TestXGlobalNoParent(t *testing.T) {
	buf := &bytes.Buffer{}
	w := tar.NewWriter(buf)
	err := w.WriteHeader(&tar.Header{
		Name:     "foo/bar",
		Typeflag: tar.TypeXGlobalHeader,
	})
	assert.NilError(t, err)
	tmpDir, err := os.MkdirTemp("", "pax-test")
	assert.NilError(t, err)
	defer os.RemoveAll(tmpDir)
	err = Untar(buf, tmpDir, nil)
	assert.NilError(t, err)

	_, err = os.Lstat(filepath.Join(tmpDir, "foo"))
	assert.Check(t, err != nil)
	assert.Check(t, errors.Is(err, os.ErrNotExist))
}

func TestReplaceFileTarWrapper(t *testing.T) {
	filesInArchive := 20
	testcases := []struct {
		doc       string
		filename  string
		modifier  TarModifierFunc
		expected  string
		fileCount int
	}{
		{
			doc:       "Modifier creates a new file",
			filename:  "newfile",
			modifier:  createModifier(t),
			expected:  "the new content",
			fileCount: filesInArchive + 1,
		},
		{
			doc:       "Modifier replaces a file",
			filename:  "file-2",
			modifier:  createOrReplaceModifier,
			expected:  "the new content",
			fileCount: filesInArchive,
		},
		{
			doc:       "Modifier replaces the last file",
			filename:  fmt.Sprintf("file-%d", filesInArchive-1),
			modifier:  createOrReplaceModifier,
			expected:  "the new content",
			fileCount: filesInArchive,
		},
		{
			doc:       "Modifier appends to a file",
			filename:  "file-3",
			modifier:  appendModifier,
			expected:  "fooo\nnext line",
			fileCount: filesInArchive,
		},
	}

	for _, testcase := range testcases {
		sourceArchive, cleanup := buildSourceArchive(t, filesInArchive)
		defer cleanup()

		resultArchive := ReplaceFileTarWrapper(
			sourceArchive,
			map[string]TarModifierFunc{testcase.filename: testcase.modifier})

		actual := readFileFromArchive(t, resultArchive, testcase.filename, testcase.fileCount, testcase.doc)
		assert.Check(t, is.Equal(testcase.expected, actual), testcase.doc)
	}
}

// TestPrefixHeaderReadable tests that files that could be created with the
// version of this package that was built with <=go17 are still readable.
func TestPrefixHeaderReadable(t *testing.T) {
	skip.If(t, runtime.GOOS != "windows" && os.Getuid() != 0, "skipping test that requires root")
	skip.If(t, userns.RunningInUserNS(), "skipping test that requires more than 010000000 UIDs, which is unlikely to be satisfied when running in userns")
	// https://gist.github.com/stevvooe/e2a790ad4e97425896206c0816e1a882#file-out-go
	var testFile = []byte("\x1f\x8b\x08\x08\x44\x21\x68\x59\x00\x03\x74\x2e\x74\x61\x72\x00\x4b\xcb\xcf\x67\xa0\x35\x30\x80\x00\x86\x06\x10\x47\x01\xc1\x37\x40\x00\x54\xb6\xb1\xa1\xa9\x99\x09\x48\x25\x1d\x40\x69\x71\x49\x62\x91\x02\xe5\x76\xa1\x79\x84\x21\x91\xd6\x80\x72\xaf\x8f\x82\x51\x30\x0a\x46\x36\x00\x00\xf0\x1c\x1e\x95\x00\x06\x00\x00")

	tmpDir, err := os.MkdirTemp("", "prefix-test")
	assert.NilError(t, err)
	defer os.RemoveAll(tmpDir)
	err = Untar(bytes.NewReader(testFile), tmpDir, nil)
	assert.NilError(t, err)

	baseName := "foo"
	pth := strings.Repeat("a", 100-len(baseName)) + "/" + baseName

	_, err = os.Lstat(filepath.Join(tmpDir, pth))
	assert.NilError(t, err)
}

func buildSourceArchive(t *testing.T, numberOfFiles int) (io.ReadCloser, func()) {
	srcDir, err := os.MkdirTemp("", "docker-test-srcDir")
	assert.NilError(t, err)

	_, err = prepareUntarSourceDirectory(numberOfFiles, srcDir, false)
	assert.NilError(t, err)

	sourceArchive, err := TarWithOptions(srcDir, &TarOptions{})
	assert.NilError(t, err)
	return sourceArchive, func() {
		os.RemoveAll(srcDir)
		sourceArchive.Close()
	}
}

func createOrReplaceModifier(path string, header *tar.Header, content io.Reader) (*tar.Header, []byte, error) {
	return &tar.Header{
		Mode:     0600,
		Typeflag: tar.TypeReg,
	}, []byte("the new content"), nil
}

func createModifier(t *testing.T) TarModifierFunc {
	return func(path string, header *tar.Header, content io.Reader) (*tar.Header, []byte, error) {
		assert.Check(t, is.Nil(content))
		return createOrReplaceModifier(path, header, content)
	}
}

func appendModifier(path string, header *tar.Header, content io.Reader) (*tar.Header, []byte, error) {
	buffer := bytes.Buffer{}
	if content != nil {
		if _, err := buffer.ReadFrom(content); err != nil {
			return nil, nil, err
		}
	}
	buffer.WriteString("\nnext line")
	return &tar.Header{Mode: 0600, Typeflag: tar.TypeReg}, buffer.Bytes(), nil
}

func readFileFromArchive(t *testing.T, archive io.ReadCloser, name string, expectedCount int, doc string) string {
	skip.If(t, runtime.GOOS != "windows" && os.Getuid() != 0, "skipping test that requires root")
	destDir, err := os.MkdirTemp("", "docker-test-destDir")
	assert.NilError(t, err)
	defer os.RemoveAll(destDir)

	err = Untar(archive, destDir, nil)
	assert.NilError(t, err)

	files, _ := os.ReadDir(destDir)
	assert.Check(t, is.Len(files, expectedCount), doc)

	content, err := os.ReadFile(filepath.Join(destDir, name))
	assert.Check(t, err)
	return string(content)
}

func TestDisablePigz(t *testing.T) {
	_, err := exec.LookPath("unpigz")
	if err != nil {
		t.Log("Test will not check full path when Pigz not installed")
	}

	os.Setenv("MOBY_DISABLE_PIGZ", "true")
	defer os.Unsetenv("MOBY_DISABLE_PIGZ")

	r := testDecompressStream(t, "gz", "gzip -f")
	// For the bufio pool
	outsideReaderCloserWrapper := r.(*ioutils.ReadCloserWrapper)
	// For the context canceller
	contextReaderCloserWrapper := outsideReaderCloserWrapper.Reader.(*ioutils.ReadCloserWrapper)

	assert.Equal(t, reflect.TypeOf(contextReaderCloserWrapper.Reader), reflect.TypeOf(&gzip.Reader{}))
}

func TestPigz(t *testing.T) {
	r := testDecompressStream(t, "gz", "gzip -f")
	// For the bufio pool
	outsideReaderCloserWrapper := r.(*ioutils.ReadCloserWrapper)
	// For the context canceller
	contextReaderCloserWrapper := outsideReaderCloserWrapper.Reader.(*ioutils.ReadCloserWrapper)

	_, err := exec.LookPath("unpigz")
	if err == nil {
		t.Log("Tested whether Pigz is used, as it installed")
		// For the command wait wrapper
		cmdWaitCloserWrapper := contextReaderCloserWrapper.Reader.(*ioutils.ReadCloserWrapper)
		assert.Equal(t, reflect.TypeOf(cmdWaitCloserWrapper.Reader), reflect.TypeOf(&io.PipeReader{}))
	} else {
		t.Log("Tested whether Pigz is not used, as it not installed")
		assert.Equal(t, reflect.TypeOf(contextReaderCloserWrapper.Reader), reflect.TypeOf(&gzip.Reader{}))
	}
}
