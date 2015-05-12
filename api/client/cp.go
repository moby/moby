package client

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdCp copies files/folders from a path on the container to a directory on the host running the command.
//
// If PATH is '-' for the first arg, the data is sent to the server from STDIN.
// If PATH is '-' for the second arg, the data is written as a tar file to STDOUT.
//
// Usage: docker cp [CONTAINER:PATH PATH|-] | [PATH|- CONTAINER:PATH]
func (cli *DockerCli) CmdCp(args ...string) error {
	cmd := cli.Subcmd("cp", "[CONTAINER:PATH PATH|-]|[PATH|- CONTAINER:PATH]", "Copy file/folder to/from a container", true)
	flPause := cmd.Bool([]string{"-pause"}, false, "Pasues the container while copying")
	cmd.Require(flag.Exact, 2)
	cmd.ParseFlags(args, true)

	args = cmd.Args()
	if args[0] == "-" && args[1] == "-" {
		return fmt.Errorf("invalid arguments, can't use `-` for source and destination")
	}

	// first let's skip some guess work.. if this is an absolute path, it's our local path
	// <container:path> should never be absolute
	if filepath.IsAbs(args[0]) {
		// assume args[0] is a local path
		s, ok := parseMaybePath(args[0], true)
		if !ok {
			return fmt.Errorf("source path is not valid: %s", args[0])
		}

		parts := strings.SplitN(args[1], ":", 2)
		if len(parts) < 2 {
			return fmt.Errorf("Invalid format for destination argument: %s", args[1])
		}
		return cli.copyPut(s, parts[1], parts[0], *flPause)
	}

	if filepath.IsAbs(args[1]) {
		// assume args[1] is a local path
		s, ok := parseMaybePath(args[1], false)
		if !ok {
			return fmt.Errorf("Invalid format for destination argument: %s", args[1])
		}
		parts := strings.SplitN(args[0], ":", 2)
		if len(parts) < 2 {
			return fmt.Errorf("Invalid format for destination argument: %s", args[1])
		}
		return cli.copyGet(s, parts[1], parts[0], *flPause)
	}

	if s, ok := parseMaybePath(args[0], true); ok {
		parts := strings.SplitN(args[1], ":", 2)
		if len(parts) < 2 {
			// ok well, let's see if arg[0] looks like container:path even though it did look like a file path
			parts = strings.SplitN(args[0], ":", 2)
			if len(parts) < 2 {
				return fmt.Errorf("Invalid format for container:path")
			}
			s, ok = parseMaybePath(args[1], false)
			if !ok {
				return fmt.Errorf("Invalid format for destination argument: %s", args[1])
			}
			return cli.copyGet(s, parts[1], parts[0], *flPause)
		}
		return cli.copyPut(s, parts[1], parts[0], *flPause)
	}

	s, ok := parseMaybePath(args[1], false)
	if !ok {
		return fmt.Errorf("Invalid format for destination argument: %s", args[1])
	}
	parts := strings.SplitN(args[0], ":", 2)
	if len(parts) < 2 {
		return fmt.Errorf("Invalid format for container:path")
	}
	return cli.copyGet(s, parts[1], parts[0], *flPause)
}

func parseMaybePath(s string, useStat bool) (string, bool) {
	if s == "-" {
		return s, true
	}
	absPath, err := filepath.Abs(s)
	if err != nil {
		return "", false
	}
	cleanPath := filepath.Clean(absPath)

	if useStat {
		if _, err := os.Stat(cleanPath); err != nil {
			return "", false
		}
	}
	return cleanPath, true
}

func (cli *DockerCli) copyGet(local, remote, container string, pause bool) error {
	cfg := &types.CopyConfig{Resource: remote, Pause: pause}
	stream, statusCode, err := cli.call("GET", "/containers/"+container+"/copy", cfg, nil)
	if stream != nil {
		defer stream.Close()
	}
	if statusCode == 404 {
		return fmt.Errorf("No such container: %v", container)
	}
	if err != nil {
		return err
	}

	if statusCode == 200 {
		if local == "-" {
			_, err = io.Copy(cli.out, stream)
		} else {
			err = archive.Untar(stream, local, &archive.TarOptions{NoLchown: true})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (cli *DockerCli) copyPut(local, remote, container string, pause bool) error {
	v := url.Values{}
	v.Set("to", remote)
	if pause {
		v.Set("pause", "1")
	}

	var data io.ReadCloser
	if local == "-" {
		data = cli.in
	} else {
		stat, err := os.Stat(local)
		if err != nil {
			return err
		}
		var filter []string
		if !stat.IsDir() {
			d, f := filepath.Split(local)
			local = d
			filter = []string{f}
		} else {
			filter = []string{filepath.Base(local)}
			local = filepath.Dir(local)
		}

		data, err = archive.TarWithOptions(local, &archive.TarOptions{
			Compression:  archive.Uncompressed,
			IncludeFiles: filter,
		})

		defer data.Close()
	}
	headers := http.Header(make(map[string][]string))
	headers.Add("Content-Type", "application/tar")

	sopts := &streamOpts{
		rawTerminal: true,
		in:          data,
		out:         cli.out,
		headers:     headers,
	}
	return cli.stream("POST", "/containers/"+container+"/copy?"+v.Encode(), sopts)
}
