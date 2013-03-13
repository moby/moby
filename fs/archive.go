package fs

import (
	"errors"
	"io"
	"io/ioutil"
	"os/exec"
)

type Compression uint32

const (
	Uncompressed Compression = iota
	Bzip2
	Gzip
)

func (compression *Compression) Flag() string {
	switch *compression {
	case Bzip2:
		return "j"
	case Gzip:
		return "z"
	}
	return ""
}

func Tar(path string, compression Compression) (io.Reader, error) {
	cmd := exec.Command("bsdtar", "-f", "-", "-C", path, "-c"+compression.Flag(), ".")
	return CmdStream(cmd)
}

func Untar(archive io.Reader, path string) error {
	cmd := exec.Command("bsdtar", "-f", "-", "-C", path, "-x")
	cmd.Stdin = archive
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(err.Error() + ": " + string(output))
	}
	return nil
}

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
	go func() {
		_, err := io.Copy(pipeW, stdout)
		if err != nil {
			pipeW.CloseWithError(err)
		}
		errText, e := ioutil.ReadAll(stderr)
		if e != nil {
			errText = []byte("(...couldn't fetch stderr: " + e.Error() + ")")
		}
		if err := cmd.Wait(); err != nil {
			// FIXME: can this block if stderr outputs more than the size of StderrPipe()'s buffer?
			pipeW.CloseWithError(errors.New(err.Error() + ": " + string(errText)))
		} else {
			pipeW.Close()
		}
	}()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return pipeR, nil
}
