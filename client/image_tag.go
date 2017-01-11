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
	if _, err := distreference.ParseNamed(source); err != nil {
		return errors.Wrapf(err, "Error parsing reference: %q is not a valid repository/tag", source)
	}

	distributionRef, err := distreference.ParseNamed(target)
	if err != nil {
		return errors.Wrapf(err, "Error parsing reference: %q is not a valid repository/tag", target)
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
