//go:build unix && !linux

package chrootarchive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/moby/sys/reexec"
	"golang.org/x/sys/unix"

	"github.com/moby/go-archive"
)

const (
	packCmd        = "chrootarchive-pack-in-chroot"
	unpackCmd      = "chrootarchive-unpack-in-chroot"
	unpackLayerCmd = "chrootarchive-unpack-layer-in-chroot"
)

func init() {
	reexec.Register(packCmd, reexecMain(packInChroot))
	reexec.Register(unpackCmd, reexecMain(unpackInChroot))
	reexec.Register(unpackLayerCmd, reexecMain(unpackLayerInChroot))
}

func reexecMain(f func(options archive.TarOptions, args ...string) error) func() {
	return func() {
		if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "root parameter is required")
			os.Exit(1)
		}

		options, err := recvOptions()
		root := os.Args[1]

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if err := syscall.Chroot(root); err != nil {
			fmt.Fprintln(
				os.Stderr,
				os.PathError{Op: "chroot", Path: root, Err: err},
			)
			os.Exit(2)
		}

		if err := f(*options, os.Args[2:]...); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
	}
}

func doUnpack(decompressedArchive io.Reader, relDest, root string, options *archive.TarOptions) error {
	optionsR, optionsW, err := os.Pipe()
	if err != nil {
		return err
	}
	defer optionsW.Close()
	defer optionsR.Close()

	stderr := bytes.NewBuffer(nil)

	cmd := reexec.Command(unpackCmd, root, relDest)
	cmd.Stdin = decompressedArchive
	cmd.Stderr = stderr
	cmd.ExtraFiles = []*os.File{
		optionsR,
	}

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("re-exec error: %w", err)
	}

	if err = json.NewEncoder(optionsW).Encode(options); err != nil {
		return fmt.Errorf("tar options encoding failed: %w", err)
	}

	if err = cmd.Wait(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	return nil
}

func doPack(relSrc, root string, options *archive.TarOptions) (io.ReadCloser, error) {
	optionsR, optionsW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer optionsW.Close()
	defer optionsR.Close()

	stderr := bytes.NewBuffer(nil)
	cmd := reexec.Command(packCmd, root, relSrc)
	cmd.ExtraFiles = []*os.File{
		optionsR,
	}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	r, w := io.Pipe()

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("re-exec error: %w", err)
	}

	go func() {
		_, _ = io.Copy(w, stdout)
		// Cleanup once stdout pipe is closed.
		if err = cmd.Wait(); err != nil {
			r.CloseWithError(fmt.Errorf("%s: %w", stderr.String(), err))
		} else {
			r.Close()
		}
	}()

	if err = json.NewEncoder(optionsW).Encode(options); err != nil {
		return nil, fmt.Errorf("tar options encoding failed: %w", err)
	}

	return r, nil
}

func doUnpackLayer(root string, layer io.Reader, options *archive.TarOptions) (int64, error) {
	var result int64
	optionsR, optionsW, err := os.Pipe()
	if err != nil {
		return 0, err
	}
	defer optionsW.Close()
	defer optionsR.Close()
	buffer := bytes.NewBuffer(nil)

	cmd := reexec.Command(unpackLayerCmd, root)
	cmd.Stdin = layer
	cmd.Stdout = buffer
	cmd.Stderr = buffer
	cmd.ExtraFiles = []*os.File{
		optionsR,
	}

	if err = cmd.Start(); err != nil {
		return 0, fmt.Errorf("re-exec error: %w", err)
	}

	if err = json.NewEncoder(optionsW).Encode(options); err != nil {
		return 0, fmt.Errorf("tar options encoding failed: %w", err)
	}

	if err = cmd.Wait(); err != nil {
		return 0, fmt.Errorf("%s: %w", buffer.String(), err)
	}

	if err = json.NewDecoder(buffer).Decode(&result); err != nil {
		return 0, fmt.Errorf("json decoding error: %w", err)
	}

	return result, nil
}

func unpackInChroot(options archive.TarOptions, args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("destination parameter is required")
	}

	relDest := args[0]

	return archive.Unpack(os.Stdin, relDest, &options)
}

func packInChroot(options archive.TarOptions, args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("source parameter is required")
	}

	relSrc := args[0]

	tb, err := archive.NewTarballer(relSrc, &options)
	if err != nil {
		return err
	}

	go tb.Do()

	_, err = io.Copy(os.Stdout, tb.Reader())

	return err
}

func unpackLayerInChroot(options archive.TarOptions, _args ...string) error {
	// We need to be able to set any perms
	_ = unix.Umask(0)

	size, err := archive.UnpackLayer("/", os.Stdin, &options)
	if err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(size)
}

func recvOptions() (*archive.TarOptions, error) {
	var options archive.TarOptions
	optionsPipe := os.NewFile(3, "tar-options")
	if optionsPipe == nil {
		return nil, fmt.Errorf("could not read tar options from the pipe")
	}
	defer optionsPipe.Close()
	err := json.NewDecoder(optionsPipe).Decode(&options)
	if err != nil {
		return &options, err
	}

	return &options, nil
}
