// +build experimental

package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/docker/engine-api/types"
	"golang.org/x/net/context"
)

// PluginInstall installs a plugin
func (cli *Client) PluginInstall(ctx context.Context, name, registryAuth string, acceptAllPermissions, noEnable bool, in io.ReadCloser, out io.Writer) error {
	headers := map[string][]string{"X-Registry-Auth": {registryAuth}}
	resp, err := cli.post(ctx, "/plugins/pull", url.Values{"name": []string{name}}, nil, headers)
	if err != nil {
		ensureReaderClosed(resp)
		return err
	}
	var privileges types.PluginPrivileges
	if err := json.NewDecoder(resp.body).Decode(&privileges); err != nil {
		return err
	}
	ensureReaderClosed(resp)

	if !acceptAllPermissions && len(privileges) > 0 {

		fmt.Fprintf(out, "Plugin %q requested the following privileges:\n", name)
		for _, privilege := range privileges {
			fmt.Fprintf(out, " - %s: %v\n", privilege.Name, privilege.Value)
		}

		fmt.Fprint(out, "Do you grant the above permissions? [y/N] ")
		reader := bufio.NewReader(in)
		line, _, err := reader.ReadLine()
		if err != nil {
			return err
		}
		if strings.ToLower(string(line)) != "y" {
			resp, _ := cli.delete(ctx, "/plugins/"+name, nil, nil)
			ensureReaderClosed(resp)
			return pluginPermissionDenied{name}
		}
	}
	if noEnable {
		return nil
	}
	return cli.PluginEnable(ctx, name)
}
