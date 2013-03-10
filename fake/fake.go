package fake

import (
	"archive/tar"
	"bytes"
	"github.com/kr/pty"
	"io"
	"math/rand"
	"os/exec"
)

func FakeTar() (io.Reader, error) {
	content := []byte("Hello world!\n")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, name := range []string{"/etc/postgres/postgres.conf", "/etc/passwd", "/var/log/postgres/postgres.conf"} {
		hdr := new(tar.Header)
		hdr.Size = int64(len(content))
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		tw.Write([]byte(content))
	}
	tw.Close()
	return buf, nil
}

func WriteFakeTar(dst io.Writer) error {
	if data, err := FakeTar(); err != nil {
		return err
	} else if _, err := io.Copy(dst, data); err != nil {
		return err
	}
	return nil
}

func RandomBytesChanged() uint {
	return uint(rand.Int31n(24 * 1024 * 1024))
}

func RandomFilesChanged() uint {
	return uint(rand.Int31n(42))
}

func RandomContainerSize() uint {
	return uint(rand.Int31n(142 * 1024 * 1024))
}

func ContainerRunning() bool {
	return false
}

func StartCommand(cmd *exec.Cmd, interactive bool) (io.WriteCloser, io.ReadCloser, error) {
	if interactive {
		term, err := pty.Start(cmd)
		if err != nil {
			return nil, nil, err
		}
		return term, term, nil
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return stdin, stdout, nil
}
