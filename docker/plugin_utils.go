package main

// This file contains the glue to connect pkg/plugins to the Docker CLI.
// It will load all of the available plugins and provide the call-back
// function that the plugin manager will invoke.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/pkg/plugin"
)

var cli *client.DockerCli

func LoadPlugins(root string, c *client.DockerCli) error {
	cli = c
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if path == root {
			return nil
		}

		pi := plugin.NewPlugin(path, PluginProcessor) // Register it
		err = pi.Start()                              // Start & load metadata
		if err != nil {
			return err
		}

		piName := pi.Config["Cmd"]
		if piName != "" {
			cli.Plugins()[piName] = pi
			dockerCommands = append(dockerCommands, command{
				piName,
				pi.Config["Description"],
			})
		} else {
			// No name so something is wrong - just stop if it still running
			pi.Stop()
		}
		return nil
	})
	return err
}

// Process any incoming request from plugin.
func PluginProcessor(m *plugin.Plugin, cmd string, buf []byte) ([]byte, error) {
	switch cmd {
	case "GetDockerHost":
		return []byte(cli.DockerHost()), nil

	case "CallDaemon":
		type callArgs struct {
			Method string
			Path   string
			Data   []byte
		}
		args := callArgs{}
		if err := json.Unmarshal(buf, &args); err != nil {
			return nil, err
		}

		rdr, _, _, err := cli.Call(args.Method, args.Path, args.Data, nil)
		if err != nil {
			return nil, err
		}

		newBuf := new(bytes.Buffer)
		if _, err = io.Copy(newBuf, rdr); err != nil {
			return nil, err
		}

		return newBuf.Bytes(), nil
	}

	return nil, fmt.Errorf("Unknown command: %s", cmd)
}
