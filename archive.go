package docker

import (
	"errors"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
)

type Archive io.Reader

type Compression uint32

const (
	Uncompressed Compression = iota
	Bzip2
	Gzip
	Xz
)

func (compression *Compression) Flag() string {
	switch *compression {
	case Bzip2:
		return "j"
	case Gzip:
		return "z"
	case Xz:
		return "J"
	}
	return ""
}

func (compression *Compression) Extension() string {
	switch *compression {
	case Uncompressed:
		return "tar"
	case Bzip2:
		return "tar.bz2"
	case Gzip:
		return "tar.gz"
	case Xz:
		return "tar.xz"
	}
	return ""
}

// Tar creates an archive from the directory at `path`, and returns it as a
// stream of bytes.
func Tar(path string, compression Compression) (io.Reader, error) {
	return TarFilter(path, compression, nil)
}

// Tar creates an archive from the directory at `path`, only including files whose relative
// paths are included in `filter`. If `filter` is nil, then all files are included.
func TarFilter(path string, compression Compression, filter []string) (io.Reader, error) {
	args := []string{"bsdtar", "-f", "-", "-C", path}
	if filter == nil {
		filter = []string{"."}
	}
	for _, f := range filter {
		args = append(args, "-c"+compression.Flag(), f)
	}
	cmd := exec.Command(args[0], args[1:]...)
	return CmdStream(cmd)
}

// Untar reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `path`.
// The archive may be compressed with one of the following algorithgms:
//  identity (uncompressed), gzip, bzip2, xz.
// FIXME: specify behavior when target path exists vs. doesn't exist.
func Untar(archive io.Reader, path string) error {
	cmd := exec.Command("bsdtar", "-f", "-", "-C", path, "-x")
	cmd.Stdin = archive
	// Hardcode locale environment for predictable outcome regardless of host configuration.
	//   (see https://github.com/dotcloud/docker/issues/355)
	cmd.Env = []string{"LANG=en_US.utf-8", "LC_ALL=en_US.utf-8"}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, output)
	}
	return nil
}

// TarUntar is a convenience function which calls Tar and Untar, with
// the output of one piped into the other. If either Tar or Untar fails,
// TarUntar aborts and returns the error.
func TarUntar(src string, filter []string, dst string) error {
	utils.Debugf("TarUntar(%s %s %s)", src, filter, dst)
	archive, err := TarFilter(src, Uncompressed, filter)
	if err != nil {
		return err
	}
	return Untar(archive, dst)
}

// UntarPath is a convenience function which looks for an archive
// at filesystem path `src`, and unpacks it at `dst`.
func UntarPath(src, dst string) error {
	if archive, err := os.Open(src); err != nil {
		return err
	} else if err := Untar(archive, dst); err != nil {
		return err
	}
	return nil
}

// CopyWithTar creates a tar archive of filesystem path `src`, and
// unpacks it at filesystem path `dst`.
// The archive is streamed directly with fixed buffering and no
// intermediary disk IO.
//
func CopyWithTar(src, dst string) error {
	srcSt, err := os.Stat(src)
	if err != nil {
		return err
	}
	var dstExists bool
	dstSt, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		dstExists = true
	}
	// Things that can go wrong if the source is a directory
	if srcSt.IsDir() {
		// The destination exists and is a regular file
		if dstExists && !dstSt.IsDir() {
			return fmt.Errorf("Can't copy a directory over a regular file")
		}
		// Things that can go wrong if the source is a regular file
	} else {
		utils.Debugf("The destination exists, it's a directory, and doesn't end in /")
		// The destination exists, it's a directory, and doesn't end in /
		if dstExists && dstSt.IsDir() && dst[len(dst)-1] != '/' {
			return fmt.Errorf("Can't copy a regular file over a directory %s |%s|", dst, dst[len(dst)-1])
		}
	}
	// Create the destination
	var dstDir string
	if dst[len(dst)-1] == '/' {
		// The destination ends in /
		//   --> dst is the holding directory
		dstDir = dst
	} else {
		// The destination doesn't end in /
		//   --> dst is the file
		dstDir = path.Dir(dst)
	}
	if !dstExists {
		// Create the holding directory if necessary
		utils.Debugf("Creating the holding directory %s", dstDir)
		if err := os.MkdirAll(dstDir, 0700); err != nil && !os.IsExist(err) {
			return err
		}
	}
	if !srcSt.IsDir() {
		return TarUntar(path.Dir(src), []string{path.Base(src)}, dstDir)
	}
	return TarUntar(src, nil, dstDir)
}

// CmdStream executes a command, and returns its stdout as a stream.
// If the command fails to run or doesn't complete successfully, an error
// will be returned, including anything written on stderr.
func CmdStream(cmd *exec.Cmd) (io.Reader, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	pipeR, pipeW := io.Pipe()
	errChan := make(chan []byte)
	// Collect stderr, we will use it in case of an error
	go func() {
		errText, e := ioutil.ReadAll(stderr)
		if e != nil {
			errText = []byte("(...couldn't fetch stderr: " + e.Error() + ")")
		}
		errChan <- errText
	}()
	// Copy stdout to the returned pipe
	go func() {
		_, err := io.Copy(pipeW, stdout)
		if err != nil {
			pipeW.CloseWithError(err)
		}
		errText := <-errChan
		if err := cmd.Wait(); err != nil {
			pipeW.CloseWithError(errors.New(err.Error() + ": " + string(errText)))
		} else {
			pipeW.Close()
		}
	}()
	// Run the command and return the pipe
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return pipeR, nil
}

// NewTempArchive reads the content of src into a temporary file, and returns the contents
// of that file as an archive. The archive can only be read once - as soon as reading completes,
// the file will be deleted.
func NewTempArchive(src Archive, dir string) (*TempArchive, error) {
	f, err := ioutil.TempFile(dir, "")
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(f, src); err != nil {
		return nil, err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	return &TempArchive{f, size}, nil
}

type TempArchive struct {
	*os.File
	Size int64 // Pre-computed from Stat().Size() as a convenience
}

func (archive *TempArchive) Read(data []byte) (int, error) {
	n, err := archive.File.Read(data)
	if err != nil {
		os.Remove(archive.File.Name())
	}
	return n, err
}
