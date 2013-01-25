package fake

import (
	"bytes"
	"math/rand"
	"io"
	"archive/tar"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/future"
	"errors"
	"os/exec"
	"strings"
	"fmt"
	"github.com/kr/pty"
)


func FakeTar() (io.Reader, error) {
	content := []byte("Hello world!\n")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, name := range []string {"/etc/postgres/postgres.conf", "/etc/passwd", "/var/log/postgres", "/var/log/postgres/postgres.conf"} {
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

type Container struct {
	*docker.Container
	Name	string
	Source	string
	Size	uint
	FilesChanged uint
	BytesChanged uint
	Running	bool
	stdoutLog *bytes.Buffer
	stdinLog *bytes.Buffer
}

func NewContainer(c *docker.Container) *Container {
	return &Container{
		Container:	c,
		Name:		c.GetUserData("name"),
		stdoutLog:	new(bytes.Buffer),
		stdinLog:	new(bytes.Buffer),
	}
}

func (c *Container) Run(command string, args []string, stdin io.ReadCloser, stdout io.Writer, tty bool) error {
	// Not thread-safe
	if c.Running {
		return errors.New("Already running")
	}
	c.Path = command
	c.Args = args
	// Reset logs
	c.stdoutLog.Reset()
	c.stdinLog.Reset()
	cmd := exec.Command(c.Path, c.Args...)
	cmd_stdin, cmd_stdout, err := startCommand(cmd, tty)
	if err != nil {
		return err
	}
	c.Running = true
	// ADD FAKE RANDOM CHANGES
	c.FilesChanged = RandomFilesChanged()
	c.BytesChanged = RandomBytesChanged()
	copy_out := future.Go(func() error {
		_, err := io.Copy(io.MultiWriter(stdout, c.stdoutLog), cmd_stdout)
		return err
	})
	future.Go(func() error {
		_, err := io.Copy(io.MultiWriter(cmd_stdin, c.stdinLog), stdin)
		cmd_stdin.Close()
		stdin.Close()
		return err
	})
	wait := future.Go(func() error {
		err := cmd.Wait()
		c.Running = false
		return err
	})
	if err := <-copy_out; err != nil {
		if c.Running {
			return err
		}
	}
	if err := <-wait; err != nil {
		if status, ok := err.(*exec.ExitError); ok {
			fmt.Fprintln(stdout, status)
			return nil
		}
		return err
	}
	return nil
}

func (c *Container) StdoutLog() io.Reader {
	return strings.NewReader(c.stdoutLog.String())
}

func (c *Container) StdinLog() io.Reader {
	return strings.NewReader(c.stdinLog.String())
}

func (c *Container) CmdString() string {
	return strings.Join(append([]string{c.Path}, c.Args...), " ")
}


func startCommand(cmd *exec.Cmd, interactive bool) (io.WriteCloser, io.ReadCloser, error) {
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


