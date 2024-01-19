package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
)

// ImageInspectWithRaw returns the image information and its raw representation.
func (cli *Client) ImageInspectWithRaw(ctx context.Context, imageID string, options image.InspectOptions) (types.ImageInspect, []byte, error) {
	if imageID == "" {
		return types.ImageInspect{}, nil, objectNotFoundError{object: "image", id: imageID}
	}
	query := url.Values{}
	if options.Platform != "" {
		query.Set("platform", strings.ToLower(options.Platform))
	}
	serverResp, err := cli.get(ctx, "/images/"+imageID+"/json", query, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return types.ImageInspect{}, nil, err
	}

	body, err := io.ReadAll(serverResp.body)
	if err != nil {
		return types.ImageInspect{}, nil, err
	}

	var response types.ImageInspect
	rdr := bytes.NewReader(body)
	err = json.NewDecoder(rdr).Decode(&response)
	return response, body, err
}
