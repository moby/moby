package client

import (
	"net/url"

	distreference "github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/reference"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// ImageTag tags an image in the docker host
func (cli *Client) ImageTag(ctx context.Context, source, target string) error {
	if _, err := parseNamed(source); err != nil {
		return err
	}

	distributionRef, err := parseNamed(target)
	if err != nil {
		return err
	}

	if _, isCanonical := distributionRef.(distreference.Canonical); isCanonical {
		return errors.New("refusing to create a tag with a digest reference")
	}

	tag := reference.GetTagFromNamedRef(distributionRef)

	query := url.Values{}
	query.Set("repo", distributionRef.Name())
	query.Set("tag", tag)

	resp, err := cli.post(ctx, "/images/"+source+"/tag", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
