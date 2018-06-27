package client

import (
	"net/http"
	"os"

	"github.com/docker/docker/client/context"
	"github.com/docker/docker/pkg/contextstore"
	"github.com/pkg/errors"
)

func loadContext(c *Client, s contextstore.Store, ctxName string) error {
	ctxMeta, err := s.GetContextMetadata(ctxName)
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve context metadata: %s", err)
	}
	ctx, err := context.Parse(ctxName, ctxMeta)
	if err != nil {
		return err
	}

	if ctx.Host != "" {
		if err = WithHost(ctx.Host)(c); err != nil {
			return err
		}
	}

	tlsCfg, err := ctx.LoadTLSConfig(s)
	if err != nil {
		return err
	}
	if tlsCfg != nil {
		if transport, ok := c.client.Transport.(*http.Transport); ok {
			transport.TLSClientConfig = tlsCfg
		} else {
			return errors.Errorf("cannot apply tls config to transport: %T", c.client.Transport)
		}
	}

	version := os.Getenv("DOCKER_API_VERSION")
	if version == "" {
		version = ctx.APIVersion
	}
	if version != "" {
		c.version = version
		c.manualOverride = true
	}
	return nil
}
