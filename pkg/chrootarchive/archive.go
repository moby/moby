package chrootarchive

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/pkg/system"
)

var chrootArchiver = &archive.Archiver{Untar: Untar}

func untar() {
	runtime.LockOSThread()
	flag.Parse()

	var options *archive.TarOptions

	if runtime.GOOS != "windows" {
		//read the options from the pipe "ExtraFiles"
		if err := json.NewDecoder(os.NewFile(3, "options")).Decode(&options); err != nil {
			fatal(err)
		}
	} else {
		if err := json.Unmarshal([]byte(os.Getenv("OPT")), &options); err != nil {
			fatal(err)
		}
	}

	if err := chroot(flag.Arg(0)); err != nil {
		fatal(err)
	}

	// Explanation of Windows difference. Windows does not support chroot.
	// untar() is a helper function for the command line in the format
	// "docker docker-untar directory input". In Windows, directory will be
	// something like <pathto>\docker-buildnnnnnnnnn. So, just use that directory
	// directly instead.
	//
	// One example of where this is used is in the docker build command where the
	// dockerfile will be unpacked to the machine on which the daemon runs.
	rootPath := "/"
	if runtime.GOOS == "windows" {
		rootPath = flag.Arg(0)
	}
	if err := archive.Unpack(os.Stdin, rootPath, options); err != nil {
		fatal(err)
	}
	// fully consume stdin in case it is zero padded
	flush(os.Stdin)
	os.Exit(0)
}

func Untar(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
	if tarArchive == nil {
		return fmt.Errorf("Empty archive")
	}
	if options == nil {
		options = &archive.TarOptions{}
	}
	if options.ExcludePatterns == nil {
		options.ExcludePatterns = []string{}
	}

	dest = filepath.Clean(dest)
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		if err := system.MkdirAll(dest, 0777); err != nil {
			return err
		}
	}

	decompressedArchive, err := archive.DecompressStream(tarArchive)
	if err != nil {
		return err
	}

	var data []byte
	var r, w *os.File
	defer decompressedArchive.Close()

	if runtime.GOOS != "windows" {
		// We can't pass a potentially large exclude list directly via cmd line
		// because we easily overrun the kernel's max argument/environment size
		// when the full image list is passed (e.g. when this is used by
		// `docker load`). We will marshall the options via a pipe to the
		// child

		// This solution won't work on Windows as it will fail in golang
		// exec_windows.go as at the lowest layer because attr.Files > 3
		r, w, err = os.Pipe()
		if err != nil {
			return fmt.Errorf("Untar pipe failure: %v", err)
		}
	} else {
		// We can't pass the exclude list directly via cmd line
		// because we easily overrun the shell max argument list length
		// when the full image list is passed (e.g. when this is used
		// by `docker load`). Instead we will add the JSON marshalled
		// and placed in the env, which has significantly larger
		// max size
		data, err = json.Marshal(options)
		if err != nil {
			return fmt.Errorf("Untar json encode: %v", err)
		}
	}

	cmd := reexec.Command("docker-untar", dest)
	cmd.Stdin = decompressedArchive

	if runtime.GOOS != "windows" {
		cmd.ExtraFiles = append(cmd.ExtraFiles, r)
		output := bytes.NewBuffer(nil)
		cmd.Stdout = output
		cmd.Stderr = output

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("Untar error on re-exec cmd: %v", err)
		}
		//write the options to the pipe for the untar exec to read
		if err := json.NewEncoder(w).Encode(options); err != nil {
			return fmt.Errorf("Untar json encode to pipe failed: %v", err)
		}
		w.Close()

		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("Untar re-exec error: %v: output: %s", err, output)
		}
		return nil
	} else {
		cmd.Env = append(cmd.Env, fmt.Sprintf("OPT=%s", data))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Untar %s %s", err, out)
		}
		return nil
	}

}

func TarUntar(src, dst string) error {
	return chrootArchiver.TarUntar(src, dst)
}

// CopyWithTar creates a tar archive of filesystem path `src`, and
// unpacks it at filesystem path `dst`.
// The archive is streamed directly with fixed buffering and no
// intermediary disk IO.
func CopyWithTar(src, dst string) error {
	return chrootArchiver.CopyWithTar(src, dst)
}

// CopyFileWithTar emulates the behavior of the 'cp' command-line
// for a single file. It copies a regular file from path `src` to
// path `dst`, and preserves all its metadata.
//
// If `dst` ends with a trailing slash '/', the final destination path
// will be `dst/base(src)`.
func CopyFileWithTar(src, dst string) (err error) {
	return chrootArchiver.CopyFileWithTar(src, dst)
}

// UntarPath is a convenience function which looks for an archive
// at filesystem path `src`, and unpacks it at `dst`.
func UntarPath(src, dst string) error {
	return chrootArchiver.UntarPath(src, dst)
}
