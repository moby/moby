//go:build !windows

package npipe

import (
	"errors"
	"net/url"

	"github.com/moby/buildkit/client/connhelper"
)

func Helper(u *url.URL) (*connhelper.ConnectionHelper, error) {
	return nil, errors.New("npipe connections are only supported on windows")
}
