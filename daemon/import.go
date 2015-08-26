package daemon

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
)

func (daemon *Daemon) ContainerDiffImport(in io.Reader, name string) error {

	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	if _, err = daemon.graph.GetDriver().ApplyDiff(container.ID, container.ImageID, archive.Reader(in)); err != nil {
		return err
	}

	return nil

}

func (daemon *Daemon) ContainerMetadataImport(in io.Reader) error {

	var containerID string

	tempdir, err := ioutil.TempDir("", "docker-import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempdir)

	if err := chrootarchive.Untar(in, tempdir, nil); err != nil {
		return err
	}

	dest := fmt.Sprintf("%s/containers", daemon.root)

	dirs, err := ioutil.ReadDir(tempdir)
	if err != nil {
		return err
	}

	for _, d := range dirs {
		if d.IsDir() {
			containerID = d.Name()
			cmd := exec.Command("cp", "-r", fmt.Sprintf("%s/%s", tempdir, containerID), dest)
			if err := cmd.Run(); err != nil {
				return err
			}
			break
		}
	}

	container, err := daemon.load(containerID)
	if err != nil {
		return err
	}

	if err := daemon.Register(container); err != nil {
		return err
	}

	if err := daemon.createRootfs(container); err != nil {
		return err
	}

	return nil

}
