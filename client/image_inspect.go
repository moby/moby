package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types/image"
)

// ImageInspectWithRaw returns the image information and its raw representation.
func (cli *Client) ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
	if imageID == "" {
		return image.InspectResponse{}, nil, objectNotFoundError{object: "image", id: imageID}
	}
	serverResp, err := cli.get(ctx, "/images/"+imageID+"/json", nil, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return image.InspectResponse{}, nil, err
	}

	body, err := io.ReadAll(serverResp.body)
	if err != nil {
		return image.InspectResponse{}, nil, err
	}

	var response image.InspectResponse
	rdr := bytes.NewReader(body)
	err = json.NewDecoder(rdr).Decode(&response)
	return response, body, err
}
