package registry // import "github.com/docker/docker/registry"

import (
	"net/url"

	"github.com/docker/distribution/registry/api/errcode"
	derrdefs "github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

func translateV2AuthError(err error) error {
	switch e := err.(type) {
	case *url.Error:
		switch e2 := e.Err.(type) {
		case errcode.Error:
			switch e2.Code {
			case errcode.ErrorCodeUnauthorized:
				return derrdefs.Unauthorized(err)
			}
		}
	}

	return err
}

func invalidParam(err error) error {
	return derrdefs.InvalidParameter(err)
}

func invalidParamf(format string, args ...interface{}) error {
	return derrdefs.InvalidParameter(errors.Errorf(format, args...))
}

func invalidParamWrapf(err error, format string, args ...interface{}) error {
	return derrdefs.InvalidParameter(errors.Wrapf(err, format, args...))
}
