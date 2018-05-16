package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"

	"github.com/docker/docker/api/types/image"
)

// ImageInspectWithRaw returns the image information and its raw representation.
func (cli *Client) ImageInspectWithRaw(ctx context.Context, imageID string) (image.Inspect, []byte, error) {
	if imageID == "" {
		return image.Inspect{}, nil, objectNotFoundError{object: "image", id: imageID}
	}
	serverResp, err := cli.get(ctx, "/images/"+imageID+"/json", nil, nil)
	if err != nil {
		return image.Inspect{}, nil, wrapResponseError(err, serverResp, "image", imageID)
	}
	defer ensureReaderClosed(serverResp)

	body, err := ioutil.ReadAll(serverResp.body)
	if err != nil {
		return image.Inspect{}, nil, err
	}

	var response image.Inspect
	rdr := bytes.NewReader(body)
	err = json.NewDecoder(rdr).Decode(&response)
	return response, body, err
}
