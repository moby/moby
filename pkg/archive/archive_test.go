package archive

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

var tmp string

func init() {
	tmp = "/tmp/"
	if runtime.GOOS == "windows" {
		tmp = os.Getenv("TEMP") + `\`
	}
}

func (s *DockerSuite) TestIsArchiveNilHeader(c *check.C) {
	out := IsArchive(nil)
	if out {
		c.Fatalf("isArchive should return false as nil is not a valid archive header")
	}
}

func (s *DockerSuite) TestIsArchiveInvalidHeader(c *check.C) {
	header := []byte{0x00, 0x01, 0x02}
	out := IsArchive(header)
	if out {
		c.Fatalf("isArchive should return false as %s is not a valid archive header", header)
	}
}

func (s *DockerSuite) TestIsArchiveBzip2(c *check.C) {
	header := []byte{0x42, 0x5A, 0x68}
	out := IsArchive(header)
	if !out {
		c.Fatalf("isArchive should return true as %s is a bz2 header", header)
	}
}

func (s *DockerSuite) TestIsArchive7zip(c *check.C) {
	header := []byte{0x50, 0x4b, 0x03, 0x04}
	out := IsArchive(header)
	if out {
		c.Fatalf("isArchive should return false as %s is a 7z header and it is not supported", header)
	}
}

func (s *DockerSuite) TestIsArchivePathDir(c *check.C) {
	cmd := exec.Command("sh", "-c", "mkdir -p /tmp/archivedir")
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.Fatalf("Fail to create an archive file for test : %s.", output)
	}
	if IsArchivePath(tmp + "archivedir") {
		c.Fatalf("Incorrectly recognised directory as an archive")
	}
}

func (s *DockerSuite) TestIsArchivePathInvalidFile(c *check.C) {
	cmd := exec.Command("sh", "-c", "dd if=/dev/zero bs=1K count=1 of=/tmp/archive && gzip --stdout /tmp/archive > /tmp/archive.gz")
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.Fatalf("Fail to create an archive file for test : %s.", output)
	}
	if IsArchivePath(tmp + "archive") {
		c.Fatalf("Incorrectly recognised invalid tar path as archive")
	}
	if IsArchivePath(tmp + "archive.gz") {
		c.Fatalf("Incorrectly recognised invalid compressed tar path as archive")
	}
}

func (s *DockerSuite) TestIsArchivePathTar(c *check.C) {
	cmd := exec.Command("sh", "-c", "touch /tmp/archivedata && tar -cf /tmp/archive /tmp/archivedata && gzip --stdout /tmp/archive > /tmp/archive.gz")
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.Fatalf("Fail to create an archive file for test : %s.", output)
	}
	if !IsArchivePath(tmp + "/archive") {
		c.Fatalf("Did not recognise valid tar path as archive")
	}
	if !IsArchivePath(tmp + "archive.gz") {
		c.Fatalf("Did not recognise valid compressed tar path as archive")
	}
}

func (s *DockerSuite) TestDecompressStreamGzip(c *check.C) {
	cmd := exec.Command("sh", "-c", "touch /tmp/archive && gzip -f /tmp/archive")
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.Fatalf("Fail to create an archive file for test : %s.", output)
	}
	archive, err := os.Open(tmp + "archive.gz")
	if err != nil {
		c.Fatalf("Fail to open file archive.gz")
	}
	defer archive.Close()

	_, err = DecompressStream(archive)
	if err != nil {
		c.Fatalf("Failed to decompress a gzip file.")
	}
}

func (s *DockerSuite) TestDecompressStreamBzip2(c *check.C) {
	cmd := exec.Command("sh", "-c", "touch /tmp/archive && bzip2 -f /tmp/archive")
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.Fatalf("Fail to create an archive file for test : %s.", output)
	}
	archive, err := os.Open(tmp + "archive.bz2")
	if err != nil {
		c.Fatalf("Fail to open file archive.bz2")
	}
	defer archive.Close()

	_, err = DecompressStream(archive)
	if err != nil {
		c.Fatalf("Failed to decompress a bzip2 file.")
	}
}

func (s *DockerSuite) TestDecompressStreamXz(c *check.C) {
	if runtime.GOOS == "windows" {
		c.Skip("Xz not present in msys2")
	}
	cmd := exec.Command("sh", "-c", "touch /tmp/archive && xz -f /tmp/archive")
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.Fatalf("Fail to create an archive file for test : %s.", output)
	}
	archive, err := os.Open(tmp + "archive.xz")
	if err != nil {
		c.Fatalf("Fail to open file archive.xz")
	}
	defer archive.Close()
	_, err = DecompressStream(archive)
	if err != nil {
		c.Fatalf("Failed to decompress an xz file.")
	}
}

func (s *DockerSuite) TestCompressStreamXzUnsuported(c *check.C) {
	dest, err := os.Create(tmp + "dest")
	if err != nil {
		c.Fatalf("Fail to create the destination file")
	}
	defer dest.Close()

	_, err = CompressStream(dest, Xz)
	if err == nil {
		c.Fatalf("Should fail as xz is unsupported for compression format.")
	}
}

func (s *DockerSuite) TestCompressStreamBzip2Unsupported(c *check.C) {
	dest, err := os.Create(tmp + "dest")
	if err != nil {
		c.Fatalf("Fail to create the destination file")
	}
	defer dest.Close()

	_, err = CompressStream(dest, Xz)
	if err == nil {
		c.Fatalf("Should fail as xz is unsupported for compression format.")
	}
}

func (s *DockerSuite) TestCompressStreamInvalid(c *check.C) {
	dest, err := os.Create(tmp + "dest")
	if err != nil {
		c.Fatalf("Fail to create the destination file")
	}
	defer dest.Close()

	_, err = CompressStream(dest, -1)
	if err == nil {
		c.Fatalf("Should fail as xz is unsupported for compression format.")
	}
}

func (s *DockerSuite) TestExtensionInvalid(c *check.C) {
	compression := Compression(-1)
	output := compression.Extension()
	if output != "" {
		c.Fatalf("The extension of an invalid compression should be an empty string.")
	}
}

func (s *DockerSuite) TestExtensionUncompressed(c *check.C) {
	compression := Uncompressed
	output := compression.Extension()
	if output != "tar" {
		c.Fatalf("The extension of an uncompressed archive should be 'tar'.")
	}
}
func (s *DockerSuite) TestExtensionBzip2(c *check.C) {
	compression := Bzip2
	output := compression.Extension()
	if output != "tar.bz2" {
		c.Fatalf("The extension of a bzip2 archive should be 'tar.bz2'")
	}
}
func (s *DockerSuite) TestExtensionGzip(c *check.C) {
	compression := Gzip
	output := compression.Extension()
	if output != "tar.gz" {
		c.Fatalf("The extension of a bzip2 archive should be 'tar.gz'")
	}
}
func (s *DockerSuite) TestExtensionXz(c *check.C) {
	compression := Xz
	output := compression.Extension()
	if output != "tar.xz" {
		c.Fatalf("The extension of a bzip2 archive should be 'tar.xz'")
	}
}

func (s *DockerSuite) TestCmdStreamLargeStderr(c *check.C) {
	cmd := exec.Command("sh", "-c", "dd if=/dev/zero bs=1k count=1000 of=/dev/stderr; echo hello")
	out, _, err := cmdStream(cmd, nil)
	if err != nil {
		c.Fatalf("Failed to start command: %s", err)
	}
	errCh := make(chan error)
	go func() {
		_, err := io.Copy(ioutil.Discard, out)
		errCh <- err
	}()
	select {
	case err := <-errCh:
		if err != nil {
			c.Fatalf("Command should not have failed (err=%.100s...)", err)
		}
	case <-time.After(5 * time.Second):
		c.Fatalf("Command did not complete in 5 seconds; probable deadlock")
	}
}

func (s *DockerSuite) TestCmdStreamBad(c *check.C) {
	// TODO Windows: Figure out why this is failing in CI but not locally
	if runtime.GOOS == "windows" {
		c.Skip("Failing on Windows CI machines")
	}
	badCmd := exec.Command("sh", "-c", "echo hello; echo >&2 error couldn\\'t reverse the phase pulser; exit 1")
	out, _, err := cmdStream(badCmd, nil)
	if err != nil {
		c.Fatalf("Failed to start command: %s", err)
	}
	if output, err := ioutil.ReadAll(out); err == nil {
		c.Fatalf("Command should have failed")
	} else if err.Error() != "exit status 1: error couldn't reverse the phase pulser\n" {
		c.Fatalf("Wrong error value (%s)", err)
	} else if s := string(output); s != "hello\n" {
		c.Fatalf("Command output should be '%s', not '%s'", "hello\\n", output)
	}
}

func (s *DockerSuite) TestCmdStreamGood(c *check.C) {
	cmd := exec.Command("sh", "-c", "echo hello; exit 0")
	out, _, err := cmdStream(cmd, nil)
	if err != nil {
		c.Fatal(err)
	}
	if output, err := ioutil.ReadAll(out); err != nil {
		c.Fatalf("Command should not have failed (err=%s)", err)
	} else if s := string(output); s != "hello\n" {
		c.Fatalf("Command output should be '%s', not '%s'", "hello\\n", output)
	}
}

func (s *DockerSuite) TestUntarPathWithInvalidDest(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "docker-archive-test")
	if err != nil {
		c.Fatal(err)
	}
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
	if err != nil {
		c.Fatal(err)
	}

	err = UntarPath(tarFile, invalidDestFolder)
	if err == nil {
		c.Fatalf("UntarPath with invalid destination path should throw an error.")
	}
}

func (s *DockerSuite) TestUntarPathWithInvalidSrc(c *check.C) {
	dest, err := ioutil.TempDir("", "docker-archive-test")
	if err != nil {
		c.Fatalf("Fail to create the destination file")
	}
	defer os.RemoveAll(dest)
	err = UntarPath("/invalid/path", dest)
	if err == nil {
		c.Fatalf("UntarPath with invalid src path should throw an error.")
	}
}

func (s *DockerSuite) TestUntarPath(c *check.C) {
	tmpFolder, err := ioutil.TempDir("", "docker-archive-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpFolder)
	srcFile := filepath.Join(tmpFolder, "src")
	tarFile := filepath.Join(tmpFolder, "src.tar")
	os.Create(filepath.Join(tmpFolder, "src"))

	destFolder := filepath.Join(tmpFolder, "dest")
	err = os.MkdirAll(destFolder, 0740)
	if err != nil {
		c.Fatalf("Fail to create the destination file")
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
	if err != nil {
		c.Fatal(err)
	}

	err = UntarPath(tarFile, destFolder)
	if err != nil {
		c.Fatalf("UntarPath shouldn't throw an error, %s.", err)
	}
	expectedFile := filepath.Join(destFolder, srcFileU)
	_, err = os.Stat(expectedFile)
	if err != nil {
		c.Fatalf("Destination folder should contain the source file but did not.")
	}
}

// Do the same test as above but with the destination as file, it should fail
func (s *DockerSuite) TestUntarPathWithDestinationFile(c *check.C) {
	tmpFolder, err := ioutil.TempDir("", "docker-archive-test")
	if err != nil {
		c.Fatal(err)
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
		c.Fatal(err)
	}
	destFile := filepath.Join(tmpFolder, "dest")
	_, err = os.Create(destFile)
	if err != nil {
		c.Fatalf("Fail to create the destination file")
	}
	err = UntarPath(tarFile, destFile)
	if err == nil {
		c.Fatalf("UntarPath should throw an error if the destination if a file")
	}
}

// Do the same test as above but with the destination folder already exists
// and the destination file is a directory
// It's working, see https://github.com/docker/docker/issues/10040
func (s *DockerSuite) TestUntarPathWithDestinationSrcFileAsFolder(c *check.C) {
	tmpFolder, err := ioutil.TempDir("", "docker-archive-test")
	if err != nil {
		c.Fatal(err)
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
		c.Fatal(err)
	}
	destFolder := filepath.Join(tmpFolder, "dest")
	err = os.MkdirAll(destFolder, 0740)
	if err != nil {
		c.Fatalf("Fail to create the destination folder")
	}
	// Let's create a folder that will has the same path as the extracted file (from tar)
	destSrcFileAsFolder := filepath.Join(destFolder, srcFileU)
	err = os.MkdirAll(destSrcFileAsFolder, 0740)
	if err != nil {
		c.Fatal(err)
	}
	err = UntarPath(tarFile, destFolder)
	if err != nil {
		c.Fatalf("UntarPath should throw not throw an error if the extracted file already exists and is a folder")
	}
}

func (s *DockerSuite) TestCopyWithTarInvalidSrc(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "docker-archive-test")
	if err != nil {
		c.Fatal(nil)
	}
	destFolder := filepath.Join(tempFolder, "dest")
	invalidSrc := filepath.Join(tempFolder, "doesnotexists")
	err = os.MkdirAll(destFolder, 0740)
	if err != nil {
		c.Fatal(err)
	}
	err = CopyWithTar(invalidSrc, destFolder)
	if err == nil {
		c.Fatalf("archiver.CopyWithTar with invalid src path should throw an error.")
	}
}

func (s *DockerSuite) TestCopyWithTarInexistentDestWillCreateIt(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "docker-archive-test")
	if err != nil {
		c.Fatal(nil)
	}
	srcFolder := filepath.Join(tempFolder, "src")
	inexistentDestFolder := filepath.Join(tempFolder, "doesnotexists")
	err = os.MkdirAll(srcFolder, 0740)
	if err != nil {
		c.Fatal(err)
	}
	err = CopyWithTar(srcFolder, inexistentDestFolder)
	if err != nil {
		c.Fatalf("CopyWithTar with an inexistent folder shouldn't fail.")
	}
	_, err = os.Stat(inexistentDestFolder)
	if err != nil {
		c.Fatalf("CopyWithTar with an inexistent folder should create it.")
	}
}

// Test CopyWithTar with a file as src
func (s *DockerSuite) TestCopyWithTarSrcFile(c *check.C) {
	folder, err := ioutil.TempDir("", "docker-archive-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(folder)
	dest := filepath.Join(folder, "dest")
	srcFolder := filepath.Join(folder, "src")
	src := filepath.Join(folder, filepath.Join("src", "src"))
	err = os.MkdirAll(srcFolder, 0740)
	if err != nil {
		c.Fatal(err)
	}
	err = os.MkdirAll(dest, 0740)
	if err != nil {
		c.Fatal(err)
	}
	ioutil.WriteFile(src, []byte("content"), 0777)
	err = CopyWithTar(src, dest)
	if err != nil {
		c.Fatalf("archiver.CopyWithTar shouldn't throw an error, %s.", err)
	}
	_, err = os.Stat(dest)
	// FIXME Check the content
	if err != nil {
		c.Fatalf("Destination file should be the same as the source.")
	}
}

// Test CopyWithTar with a folder as src
func (s *DockerSuite) TestCopyWithTarSrcFolder(c *check.C) {
	folder, err := ioutil.TempDir("", "docker-archive-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(folder)
	dest := filepath.Join(folder, "dest")
	src := filepath.Join(folder, filepath.Join("src", "folder"))
	err = os.MkdirAll(src, 0740)
	if err != nil {
		c.Fatal(err)
	}
	err = os.MkdirAll(dest, 0740)
	if err != nil {
		c.Fatal(err)
	}
	ioutil.WriteFile(filepath.Join(src, "file"), []byte("content"), 0777)
	err = CopyWithTar(src, dest)
	if err != nil {
		c.Fatalf("archiver.CopyWithTar shouldn't throw an error, %s.", err)
	}
	_, err = os.Stat(dest)
	// FIXME Check the content (the file inside)
	if err != nil {
		c.Fatalf("Destination folder should contain the source file but did not.")
	}
}

func (s *DockerSuite) TestCopyFileWithTarInvalidSrc(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "docker-archive-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tempFolder)
	destFolder := filepath.Join(tempFolder, "dest")
	err = os.MkdirAll(destFolder, 0740)
	if err != nil {
		c.Fatal(err)
	}
	invalidFile := filepath.Join(tempFolder, "doesnotexists")
	err = CopyFileWithTar(invalidFile, destFolder)
	if err == nil {
		c.Fatalf("archiver.CopyWithTar with invalid src path should throw an error.")
	}
}

func (s *DockerSuite) TestCopyFileWithTarInexistentDestWillCreateIt(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "docker-archive-test")
	if err != nil {
		c.Fatal(nil)
	}
	defer os.RemoveAll(tempFolder)
	srcFile := filepath.Join(tempFolder, "src")
	inexistentDestFolder := filepath.Join(tempFolder, "doesnotexists")
	_, err = os.Create(srcFile)
	if err != nil {
		c.Fatal(err)
	}
	err = CopyFileWithTar(srcFile, inexistentDestFolder)
	if err != nil {
		c.Fatalf("CopyWithTar with an inexistent folder shouldn't fail.")
	}
	_, err = os.Stat(inexistentDestFolder)
	if err != nil {
		c.Fatalf("CopyWithTar with an inexistent folder should create it.")
	}
	// FIXME Test the src file and content
}

func (s *DockerSuite) TestCopyFileWithTarSrcFolder(c *check.C) {
	folder, err := ioutil.TempDir("", "docker-archive-copyfilewithtar-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(folder)
	dest := filepath.Join(folder, "dest")
	src := filepath.Join(folder, "srcfolder")
	err = os.MkdirAll(src, 0740)
	if err != nil {
		c.Fatal(err)
	}
	err = os.MkdirAll(dest, 0740)
	if err != nil {
		c.Fatal(err)
	}
	err = CopyFileWithTar(src, dest)
	if err == nil {
		c.Fatalf("CopyFileWithTar should throw an error with a folder.")
	}
}

func (s *DockerSuite) TestCopyFileWithTarSrcFile(c *check.C) {
	folder, err := ioutil.TempDir("", "docker-archive-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(folder)
	dest := filepath.Join(folder, "dest")
	srcFolder := filepath.Join(folder, "src")
	src := filepath.Join(folder, filepath.Join("src", "src"))
	err = os.MkdirAll(srcFolder, 0740)
	if err != nil {
		c.Fatal(err)
	}
	err = os.MkdirAll(dest, 0740)
	if err != nil {
		c.Fatal(err)
	}
	ioutil.WriteFile(src, []byte("content"), 0777)
	err = CopyWithTar(src, dest+"/")
	if err != nil {
		c.Fatalf("archiver.CopyFileWithTar shouldn't throw an error, %s.", err)
	}
	_, err = os.Stat(dest)
	if err != nil {
		c.Fatalf("Destination folder should contain the source file but did not.")
	}
}

func (s *DockerSuite) TestTarFiles(c *check.C) {
	// TODO Windows: Figure out how to port this test.
	if runtime.GOOS == "windows" {
		c.Skip("Failing on Windows")
	}
	// try without hardlinks
	if err := checkNoChanges(1000, false); err != nil {
		c.Fatal(err)
	}
	// try with hardlinks
	if err := checkNoChanges(1000, true); err != nil {
		c.Fatal(err)
	}
}

func checkNoChanges(fileNum int, hardlinks bool) error {
	srcDir, err := ioutil.TempDir("", "docker-test-srcDir")
	if err != nil {
		return err
	}
	defer os.RemoveAll(srcDir)

	destDir, err := ioutil.TempDir("", "docker-test-destDir")
	if err != nil {
		return err
	}
	defer os.RemoveAll(destDir)

	_, err = prepareUntarSourceDirectory(fileNum, srcDir, hardlinks)
	if err != nil {
		return err
	}

	err = TarUntar(srcDir, destDir)
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

func tarUntar(c *check.C, origin string, options *TarOptions) ([]Change, error) {
	archive, err := TarWithOptions(origin, options)
	if err != nil {
		c.Fatal(err)
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

	tmp, err := ioutil.TempDir("", "docker-test-untar")
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

func (s *DockerSuite) TestTarUntar(c *check.C) {
	// TODO Windows: Figure out how to fix this test.
	if runtime.GOOS == "windows" {
		c.Skip("Failing on Windows")
	}
	origin, err := ioutil.TempDir("", "docker-test-untar-origin")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(origin)
	if err := ioutil.WriteFile(filepath.Join(origin, "1"), []byte("hello world"), 0700); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(origin, "2"), []byte("welcome!"), 0700); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(origin, "3"), []byte("will be ignored"), 0700); err != nil {
		c.Fatal(err)
	}

	for _, com := range []Compression{
		Uncompressed,
		Gzip,
	} {
		changes, err := tarUntar(c, origin, &TarOptions{
			Compression:     com,
			ExcludePatterns: []string{"3"},
		})

		if err != nil {
			c.Fatalf("Error tar/untar for compression %s: %s", com.Extension(), err)
		}

		if len(changes) != 1 || changes[0].Path != "/3" {
			c.Fatalf("Unexpected differences after tarUntar: %v", changes)
		}
	}
}

func (s *DockerSuite) TestTarWithOptions(c *check.C) {
	// TODO Windows: Figure out how to fix this test.
	if runtime.GOOS == "windows" {
		c.Skip("Failing on Windows")
	}
	origin, err := ioutil.TempDir("", "docker-test-untar-origin")
	if err != nil {
		c.Fatal(err)
	}
	if _, err := ioutil.TempDir(origin, "folder"); err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(origin)
	if err := ioutil.WriteFile(filepath.Join(origin, "1"), []byte("hello world"), 0700); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(origin, "2"), []byte("welcome!"), 0700); err != nil {
		c.Fatal(err)
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
		changes, err := tarUntar(c, origin, testCase.opts)
		if err != nil {
			c.Fatalf("Error tar/untar when testing inclusion/exclusion: %s", err)
		}
		if len(changes) != testCase.numChanges {
			c.Errorf("Expected %d changes, got %d for %+v:",
				testCase.numChanges, len(changes), testCase.opts)
		}
	}
}

// Some tar archives such as http://haproxy.1wt.eu/download/1.5/src/devel/haproxy-1.5-dev21.tar.gz
// use PAX Global Extended Headers.
// Failing prevents the archives from being uncompressed during ADD
func (s *DockerSuite) TestTypeXGlobalHeaderDoesNotFail(c *check.C) {
	hdr := tar.Header{Typeflag: tar.TypeXGlobalHeader}
	tmpDir, err := ioutil.TempDir("", "docker-test-archive-pax-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	err = createTarFile(filepath.Join(tmpDir, "pax_global_header"), tmpDir, &hdr, nil, true, nil, false)
	if err != nil {
		c.Fatal(err)
	}
}

// Some tar have both GNU specific (huge uid) and Ustar specific (long name) things.
// Not supposed to happen (should use PAX instead of Ustar for long name) but it does and it should still work.
func (s *DockerSuite) TestUntarUstarGnuConflict(c *check.C) {
	f, err := os.Open("testdata/broken.tar")
	if err != nil {
		c.Fatal(err)
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
			c.Fatal(err)
		}
		if hdr.Name == "root/.cpanm/work/1395823785.24209/Plack-1.0030/blib/man3/Plack::Middleware::LighttpdScriptNameFix.3pm" {
			found = true
			break
		}
	}
	if !found {
		c.Fatalf("%s not found in the archive", "root/.cpanm/work/1395823785.24209/Plack-1.0030/blib/man3/Plack::Middleware::LighttpdScriptNameFix.3pm")
	}
}

func prepareUntarSourceDirectory(numberOfFiles int, targetPath string, makeLinks bool) (int, error) {
	fileData := []byte("fooo")
	for n := 0; n < numberOfFiles; n++ {
		fileName := fmt.Sprintf("file-%d", n)
		if err := ioutil.WriteFile(filepath.Join(targetPath, fileName), fileData, 0700); err != nil {
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

func (s *DockerSuite) BenchmarkTarUntar(c *check.C) {
	origin, err := ioutil.TempDir("", "docker-test-untar-origin")
	if err != nil {
		c.Fatal(err)
	}
	tempDir, err := ioutil.TempDir("", "docker-test-untar-destination")
	if err != nil {
		c.Fatal(err)
	}
	target := filepath.Join(tempDir, "dest")
	n, err := prepareUntarSourceDirectory(100, origin, false)
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(origin)
	defer os.RemoveAll(tempDir)

	c.ResetTimer()
	c.SetBytes(int64(n))
	for n := 0; n < c.N; n++ {
		err := TarUntar(origin, target)
		if err != nil {
			c.Fatal(err)
		}
		os.RemoveAll(target)
	}
}

func (s *DockerSuite) BenchmarkTarUntarWithLinks(c *check.C) {
	origin, err := ioutil.TempDir("", "docker-test-untar-origin")
	if err != nil {
		c.Fatal(err)
	}
	tempDir, err := ioutil.TempDir("", "docker-test-untar-destination")
	if err != nil {
		c.Fatal(err)
	}
	target := filepath.Join(tempDir, "dest")
	n, err := prepareUntarSourceDirectory(100, origin, true)
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(origin)
	defer os.RemoveAll(tempDir)

	c.ResetTimer()
	c.SetBytes(int64(n))
	for n := 0; n < c.N; n++ {
		err := TarUntar(origin, target)
		if err != nil {
			c.Fatal(err)
		}
		os.RemoveAll(target)
	}
}

func (s *DockerSuite) TestUntarInvalidFilenames(c *check.C) {
	// TODO Windows: Figure out how to fix this test.
	if runtime.GOOS == "windows" {
		c.Skip("Passes but hits breakoutError: platform and architecture is not supported")
	}
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
			c.Fatalf("i=%d. %v", i, err)
		}
	}
}

func (s *DockerSuite) TestUntarHardlinkToSymlink(c *check.C) {
	// TODO Windows. There may be a way of running this, but turning off for now
	if runtime.GOOS == "windows" {
		c.Skip("hardlinks on Windows")
	}
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
			c.Fatalf("i=%d. %v", i, err)
		}
	}
}

func (s *DockerSuite) TestUntarInvalidHardlink(c *check.C) {
	// TODO Windows. There may be a way of running this, but turning off for now
	if runtime.GOOS == "windows" {
		c.Skip("hardlinks on Windows")
	}
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
			c.Fatalf("i=%d. %v", i, err)
		}
	}
}

func (s *DockerSuite) TestUntarInvalidSymlink(c *check.C) {
	// TODO Windows. There may be a way of running this, but turning off for now
	if runtime.GOOS == "windows" {
		c.Skip("hardlinks on Windows")
	}
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
			c.Fatalf("i=%d. %v", i, err)
		}
	}
}

func (s *DockerSuite) TestTempArchiveCloseMultipleTimes(c *check.C) {
	reader := ioutil.NopCloser(strings.NewReader("hello"))
	tempArchive, err := NewTempArchive(reader, "")
	buf := make([]byte, 10)
	n, err := tempArchive.Read(buf)
	if n != 5 {
		c.Fatalf("Expected to read 5 bytes. Read %d instead", n)
	}
	for i := 0; i < 3; i++ {
		if err = tempArchive.Close(); err != nil {
			c.Fatalf("i=%d. Unexpected error closing temp archive: %v", i, err)
		}
	}
}
