package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/pkg/archive"
)

type copyDirection int

const (
	fromContainer copyDirection = (1 << iota)
	toContainer
	acrossContainers = fromContainer | toContainer
)

// CmdCp handles copying files or directories to/from/across containers.
func (cli *DockerCli) CmdCp(args ...string) error {
	cmd := cli.Subcmd("cp", "[SRC_CONTAINER:]SRC_PATH [DST_CONTAINER:]DST_PATH", "Copy files between containers or a local path", true)
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 2 {
		cmd.Usage()
		return nil
	}

	var (
		srcPath      = cmd.Arg(0)
		dstPath      = cmd.Arg(1)
		srcContainer string
		dstContainer string
		direction    copyDirection
	)

	if strings.Contains(srcPath, ":") {
		// Copy from a container.
		direction |= fromContainer
		srcInfo := strings.SplitN(srcPath, ":", 2)
		srcContainer, srcPath = srcInfo[0], srcInfo[1]
	}

	if strings.Contains(dstPath, ":") {
		// Copy to a container.
		direction |= toContainer
		dstInfo := strings.SplitN(dstPath, ":", 2)
		dstContainer, dstPath = dstInfo[0], dstInfo[1]
	}

	switch direction {
	case fromContainer:
		// Copy from a container to a local path.
		return cli.copyFromContainer(srcContainer, srcPath, dstPath)
	case toContainer:
		// Copy from a local path to a container.
		return cli.copyToContainer(srcPath, dstContainer, dstPath)
	case acrossContainers:
		// Copy from one container to another.
		return cli.copyAcrossContainers(srcContainer, srcPath, dstContainer, dstPath)
	default:
		// User didn't specify any container.
		return fmt.Errorf("error: Wrong path format. Must specify a source and/or destination container")
	}
}

type containerPathStat struct {
	Name    string      `json:"name"`
	AbsPath string      `json:"absPath"`
	Size    int64       `json:"size"`
	Mode    os.FileMode `json:"mode"`
	Mtime   time.Time   `json:"mtime"`
}

func (cli *DockerCli) statContainerPath(containerName, path string) (containerPathStat, error) {
	var stat containerPathStat

	query := make(url.Values, 1)
	query.Set("path", path)

	urlStr := fmt.Sprintf("/containers/%s/stat-path?%s", containerName, query.Encode())

	response, statusCode, err := cli.call("GET", urlStr, nil, false)
	if err != nil {
		return stat, err
	}

	defer response.Close()

	if statusCode != 200 {
		return stat, fmt.Errorf("unexpected status code from daemon: %d", statusCode)
	}

	err = json.NewDecoder(response).Decode(&stat)

	return stat, err
}

func resolveLocalPath(localPath string) (absPath string, err error) {
	if absPath, err = filepath.Abs(localPath); err != nil {
		return
	}

	return archive.PreserveTrailingDotOrSeparator(absPath, localPath), nil
}

func (cli *DockerCli) copyFromContainer(srcContainer, srcPath, dstPath string) error {
	// Get an absolute destination path.
	dstPath, err := resolveLocalPath(dstPath)
	if err != nil {
		return err
	}

	// Start by stat-ing the source path in the container.
	srcStat, err := cli.statContainerPath(srcContainer, srcPath)
	if err != nil {
		// Some error, like container or resource does not
		// exist, so we can't perform the copy operation.
		return err
	}

	query := make(url.Values, 1)
	query.Set("path", srcPath)

	urlStr := fmt.Sprintf("/containers/%s/archive-path?%s", srcContainer, query.Encode())

	response, statusCode, err := cli.call("GET", urlStr, nil, false)
	if err != nil {
		return err
	}

	defer response.Close()

	if statusCode != 200 {
		return fmt.Errorf("unexpected status code from daemon: %d", statusCode)
	}

	// Prepare source copy info.
	srcInfo := archive.CopyInfo{
		Path:   srcPath,
		Exists: true,
		IsDir:  srcStat.Mode.IsDir(),
	}

	return archive.CopyTo(response, srcInfo, dstPath)
}

func (cli *DockerCli) copyToContainer(srcPath, dstContainer, dstPath string) error {
	// Get an absolute source path.
	srcPath, err := resolveLocalPath(srcPath)
	if err != nil {
		return err
	}

	// Prepare source copy info.
	srcInfo, err := archive.CopyInfoStatPath(srcPath, true)
	if err != nil {
		return err
	}

	// Prepare destination copy info by stat-ing the container path.
	dstInfo := archive.CopyInfo{Path: dstPath}
	dstStat, err := cli.statContainerPath(dstContainer, dstPath)
	if err == nil {
		dstInfo.Exists, dstInfo.IsDir = true, dstStat.Mode.IsDir()
	} else if strings.Contains(err.Error(), archive.ErrNotDirectory.Error()) {
		// The destination was asserted to be a directory but exists as a file.
		return err
	}
	// Ignore any other error and assume that the parent directory of the
	// destination path exists, in which case the copy may still succeed.

	content, err := archive.TarResource(srcPath)
	if err != nil {
		return err
	}
	defer content.Close()

	dstDir, copyContent, err := archive.PrepareArchiveCopy(content, srcInfo, dstInfo)
	if err != nil {
		return err
	}

	query := make(url.Values, 1)
	query.Set("dstDir", dstDir)

	urlStr := fmt.Sprintf("/containers/%s/extract-to-dir?%s", dstContainer, query.Encode())

	return cli.stream("PUT", urlStr, copyContent, nil, nil)
}

func (cli *DockerCli) copyAcrossContainers(srcContainer, srcPath, dstContainer, dstPath string) error {
	query := make(url.Values, 3)
	query.Set("srcContainer", srcContainer)
	query.Set("srcPath", srcPath)
	query.Set("dstPath", dstPath)

	urlStr := fmt.Sprintf("/containers/%s/copy-across?%s", dstContainer, query.Encode())

	response, statusCode, err := cli.call("POST", urlStr, nil, false)
	if err != nil {
		return err
	}

	defer response.Close()

	if statusCode != 200 {
		return fmt.Errorf("unexpected status code from daemon: %d", statusCode)
	}

	return nil
}
