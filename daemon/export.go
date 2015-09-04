package daemon

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/docker/docker/pkg/archive"
)

// ContainerExport writes the contents of the container to the given
// writer. An error is returned if the container cannot be found.
func (daemon *Daemon) ContainerExport(name string, out io.Writer) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	data, err := container.export()
	if err != nil {
		return fmt.Errorf("%s: %s", name, err)
	}
	defer data.Close()

	// Stream the entire contents of the container (basically a volatile snapshot)
	if _, err := io.Copy(out, data); err != nil {
		return fmt.Errorf("%s: %s", name, err)
	}
	return nil
}

func (daemon *Daemon) ContainerDiffExport(name string, out io.Writer) error {

	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	rwTar, err := container.ExportRw()
	if err != nil {
		return err
	}
	defer func() {
		if rwTar != nil {
			rwTar.Close()
		}
	}()

	if _, err = io.Copy(out, rwTar); err != nil {
		return err
	}

	return nil

}

func (daemon *Daemon) ContainerMetadataExport(name string, out io.Writer) error {

	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	tempdir, err := ioutil.TempDir("", "docker-export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempdir)

	src := fmt.Sprintf("%s/containers/%s", daemon.root, container.ID)

	cmd := exec.Command("cp", "-r", src, tempdir)
	if err := cmd.Run(); err != nil {
		return err
	}

	fs, err := archive.Tar(tempdir, archive.Uncompressed)
	if err != nil {
		return err
	}
	defer fs.Close()

	if _, err = io.Copy(out, fs); err != nil {
		return err
	}

	return nil

}
