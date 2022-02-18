package registry // import "github.com/moby/moby/registry"

import (
	"net/url"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/moby/moby/errdefs"
)

func translateV2AuthError(err error) error {
	switch e := err.(type) {
	case *url.Error:
		switch e2 := e.Err.(type) {
		case errcode.Error:
			switch e2.Code {
			case errcode.ErrorCodeUnauthorized:
				return errdefs.Unauthorized(err)
			}
		}
	}

	return err
}
