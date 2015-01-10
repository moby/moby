package client

import (
	"fmt"
	"net/url"
	"strings"

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

func (cli *DockerCli) copyFromContainer(srcContainer, srcPath, dstPath string) error {
	query := make(url.Values, 1)
	query.Set("path", srcPath)

	urlStr := fmt.Sprintf("/containers/%s/copy?%s", srcContainer, query.Encode())

	response, statusCode, err := cli.call("GET", urlStr, nil, false)
	if err != nil {
		return err
	}

	defer response.Close()

	if statusCode != 200 {
		return fmt.Errorf("unexpected status code from daemon: %d", statusCode)
	}

	return archive.CopyTo(response, dstPath)
}

func (cli *DockerCli) copyToContainer(srcPath, dstContainer, dstPath string) error {
	query := make(url.Values, 1)
	query.Set("dstPath", dstPath)

	urlStr := fmt.Sprintf("/containers/%s/copy?%s", dstContainer, query.Encode())

	content, err := archive.CopyFrom(srcPath)
	if err != nil {
		return err
	}
	defer content.Close()

	return cli.stream("POST", urlStr, content, nil, nil)
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
