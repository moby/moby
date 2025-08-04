package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/distribution/reference"
)

// ImageTag tags an image in the docker host
func (cli *Client) ImageTag(ctx context.Context, source, target string) error {
	if _, err := reference.ParseAnyReference(source); err != nil {
		return fmt.Errorf("error parsing reference: %q is not a valid repository/tag: %w", source, err)
	}

	ref, err := reference.ParseNormalizedNamed(target)
	if err != nil {
		return fmt.Errorf("error parsing reference: %q is not a valid repository/tag: %w", target, err)
	}

	if _, isCanonical := ref.(reference.Canonical); isCanonical {
		return errors.New("refusing to create a tag with a digest reference")
	}

	ref = reference.TagNameOnly(ref)

	query := url.Values{}
	query.Set("repo", ref.Name())
	if tagged, ok := ref.(reference.Tagged); ok {
		query.Set("tag", tagged.Tag())
	}

	resp, err := cli.post(ctx, "/images/"+source+"/tag", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
