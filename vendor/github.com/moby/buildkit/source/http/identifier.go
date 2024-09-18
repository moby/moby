package http

import (
	"github.com/moby/buildkit/solver/llbsolver/provenance"
	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	"github.com/moby/buildkit/source"
	srctypes "github.com/moby/buildkit/source/types"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

func NewHTTPIdentifier(str string, tls bool) (*HTTPIdentifier, error) {
	proto := "https://"
	if !tls {
		proto = "http://"
	}
	return &HTTPIdentifier{TLS: tls, URL: proto + str}, nil
}

type HTTPIdentifier struct {
	TLS      bool
	URL      string
	Checksum digest.Digest
	Filename string
	Perm     int
	UID      int
	GID      int
}

var _ source.Identifier = (*HTTPIdentifier)(nil)

func (id *HTTPIdentifier) Scheme() string {
	if id.TLS {
		return srctypes.HTTPSScheme
	}
	return srctypes.HTTPScheme
}

func (id *HTTPIdentifier) Capture(c *provenance.Capture, pin string) error {
	dgst, err := digest.Parse(pin)
	if err != nil {
		return errors.Wrapf(err, "failed to parse HTTP digest %s", pin)
	}
	c.AddHTTP(provenancetypes.HTTPSource{
		URL:    id.URL,
		Digest: dgst,
	})
	return nil
}
