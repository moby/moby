package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"

	"github.com/docker/docker/api/types/plugins"
)

// PluginInspectWithRaw inspects an existing plugin
func (cli *Client) PluginInspectWithRaw(ctx context.Context, name string) (*plugins.Plugin, []byte, error) {
	if name == "" {
		return nil, nil, objectNotFoundError{object: "plugin", id: name}
	}
	resp, err := cli.get(ctx, "/plugins/"+name+"/json", nil, nil)
	if err != nil {
		return nil, nil, wrapResponseError(err, resp, "plugin", name)
	}

	defer ensureReaderClosed(resp)
	body, err := ioutil.ReadAll(resp.body)
	if err != nil {
		return nil, nil, err
	}
	var p plugins.Plugin
	rdr := bytes.NewReader(body)
	err = json.NewDecoder(rdr).Decode(&p)
	return &p, body, err
}
