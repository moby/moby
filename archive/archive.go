package archive

import (
	"archive/tar"
	"bytes"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
)

type Archive io.Reader

type Compression int

type TarOptions struct {
	Includes    []string
	Excludes    []string
	Recursive   bool
	Compression Compression
	CreateFiles []string
}

const (
	Uncompressed Compression = iota
	Bzip2
	Gzip
	Xz
)

func DetectCompression(source []byte) Compression {
	sourceLen := len(source)
	for compression, m := range map[Compression][]byte{
		Bzip2: {0x42, 0x5A, 0x68},
		Gzip:  {0x1F, 0x8B, 0x08},
		Xz:    {0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00},
	} {
		fail := false
		if len(m) > sourceLen {
			utils.Debugf("Len too short")
			continue
		}
		i := 0
		for _, b := range m {
			if b != source[i] {
				fail = true
				break
			}
			i++
		}
		if !fail {
			return compression
		}
	}
	return Uncompressed
}

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
	return TarFilter(path, &TarOptions{Recursive: true, Compression: compression})
}

func escapeName(name string) string {
	escaped := make([]byte, 0)
	for i, c := range []byte(name) {
		if i == 0 && c == '/' {
			continue
		}
		// all printable chars except "-" which is 0x2d
		if (0x20 <= c && c <= 0x7E) && c != 0x2d {
			escaped = append(escaped, c)
		} else {
			escaped = append(escaped, fmt.Sprintf("\\%03o", c)...)
		}
	}
	return string(escaped)
}

// Tar creates an archive from the directory at `path`, only including files whose relative
// paths are included in `filter`. If `filter` is nil, then all files are included.
func TarFilter(path string, options *TarOptions) (io.Reader, error) {
	args := []string{"tar", "--numeric-owner", "-f", "-", "-C", path, "-T", "-"}
	if options.Includes == nil {
		options.Includes = []string{"."}
	}
	args = append(args, "-c"+options.Compression.Flag())

	for _, exclude := range options.Excludes {
		args = append(args, fmt.Sprintf("--exclude=%s", exclude))
	}

	if !options.Recursive {
		args = append(args, "--no-recursion")
	}

	files := ""
	for _, f := range options.Includes {
		files = files + escapeName(f) + "\n"
	}

	tmpDir := ""

	if options.CreateFiles != nil {
		var err error // Can't use := here or we override the outer tmpDir
		tmpDir, err = ioutil.TempDir("", "docker-tar")
		if err != nil {
			return nil, err
		}

		files = files + "-C" + tmpDir + "\n"
		for _, f := range options.CreateFiles {
			path := filepath.Join(tmpDir, f)
			err := os.MkdirAll(filepath.Dir(path), 0600)
			if err != nil {
				return nil, err
			}

			if file, err := os.OpenFile(path, os.O_CREATE, 0600); err != nil {
				return nil, err
			} else {
				file.Close()
			}
			files = files + escapeName(f) + "\n"
		}
	}

	return CmdStream(exec.Command(args[0], args[1:]...), &files, func() {
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
	})
}

// Untar reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `path`.
// The archive may be compressed with one of the following algorithms:
//  identity (uncompressed), gzip, bzip2, xz.
// FIXME: specify behavior when target path exists vs. doesn't exist.
func Untar(archive io.Reader, path string, options *TarOptions) error {
	if archive == nil {
		return fmt.Errorf("Empty archive")
	}

	buf := make([]byte, 10)
	totalN := 0
	for totalN < 10 {
		n, err := archive.Read(buf[totalN:])
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("Tarball too short")
			}
			return err
		}
		totalN += n
		utils.Debugf("[tar autodetect] n: %d", n)
	}

	compression := DetectCompression(buf)

	utils.Debugf("Archive compression detected: %s", compression.Extension())
	args := []string{"--numeric-owner", "-f", "-", "-C", path, "-x" + compression.Flag()}

	if options != nil {
		for _, exclude := range options.Excludes {
			args = append(args, fmt.Sprintf("--exclude=%s", exclude))
		}
	}

	cmd := exec.Command("tar", args...)
	cmd.Stdin = io.MultiReader(bytes.NewReader(buf), archive)
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
	archive, err := TarFilter(src, &TarOptions{Compression: Uncompressed, Includes: filter, Recursive: true})
	if err != nil {
		return err
	}
	return Untar(archive, dst, nil)
}

// UntarPath is a convenience function which looks for an archive
// at filesystem path `src`, and unpacks it at `dst`.
func UntarPath(src, dst string) error {
	if archive, err := os.Open(src); err != nil {
		return err
	} else if err := Untar(archive, dst, nil); err != nil {
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
	if !srcSt.IsDir() {
		return CopyFileWithTar(src, dst)
	}
	// Create dst, copy src's content into it
	utils.Debugf("Creating dest directory: %s", dst)
	if err := os.MkdirAll(dst, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	utils.Debugf("Calling TarUntar(%s, %s)", src, dst)
	return TarUntar(src, nil, dst)
}

// CopyFileWithTar emulates the behavior of the 'cp' command-line
// for a single file. It copies a regular file from path `src` to
// path `dst`, and preserves all its metadata.
//
// If `dst` ends with a trailing slash '/', the final destination path
// will be `dst/base(src)`.
func CopyFileWithTar(src, dst string) error {
	utils.Debugf("CopyFileWithTar(%s, %s)", src, dst)
	srcSt, err := os.Stat(src)
	if err != nil {
		return err
	}
	if srcSt.IsDir() {
		return fmt.Errorf("Can't copy a directory")
	}
	// Clean up the trailing /
	if dst[len(dst)-1] == '/' {
		dst = path.Join(dst, filepath.Base(src))
	}
	// Create the holding directory if necessary
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil && !os.IsExist(err) {
		return err
	}
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	hdr, err := tar.FileInfoHeader(srcSt, "")
	if err != nil {
		return err
	}
	hdr.Name = filepath.Base(dst)
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	if _, err := io.Copy(tw, srcF); err != nil {
		return err
	}
	tw.Close()
	return Untar(buf, filepath.Dir(dst), nil)
}

// CmdStream executes a command, and returns its stdout as a stream.
// If the command fails to run or doesn't complete successfully, an error
// will be returned, including anything written on stderr.
func CmdStream(cmd *exec.Cmd, input *string, atEnd func()) (io.Reader, error) {
	if input != nil {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			if atEnd != nil {
				atEnd()
			}
			return nil, err
		}
		// Write stdin if any
		go func() {
			_, _ = stdin.Write([]byte(*input))
			stdin.Close()
		}()
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		if atEnd != nil {
			atEnd()
		}
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		if atEnd != nil {
			atEnd()
		}
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
			pipeW.CloseWithError(fmt.Errorf("%s: %s", err, errText))
		} else {
			pipeW.Close()
		}
		if atEnd != nil {
			atEnd()
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
