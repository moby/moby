package context

import (
	"crypto/tls"
	"crypto/x509"

	"github.com/docker/docker/pkg/contextstore"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/pkg/errors"
)

// Context is a typed wrapper around a context-store context
type Context struct {
	Name          string
	Host          string
	SkipTLSVerify bool
	APIVersion    string
}

// TLSData holds ca/cert/key raw data
type TLSData struct {
	CA   []byte
	Key  []byte
	Cert []byte
}

// LoadTLSData loads TLS materials for the context
func (c *Context) LoadTLSData(s contextstore.Store) (*TLSData, error) {
	tlsFiles, err := s.ListContextTLSFiles(c.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve context tls info")
	}
	if epTLSFiles, ok := tlsFiles[dockerEndpointKey]; ok {
		var tlsData TLSData
		for _, f := range epTLSFiles {
			data, err := s.GetContextTLSData(c.Name, dockerEndpointKey, f)
			if err != nil {
				return nil, errors.Wrap(err, "failed to retrieve context tls info")
			}
			switch f {
			case caKey:
				tlsData.CA = data
			case certKey:
				tlsData.Cert = data
			case keyKey:
				tlsData.Key = data
			}
		}
		return &tlsData, nil
	}
	return nil, nil
}

// LoadTLSConfig extracts a context docker endpoint TLS config
func (c *Context) LoadTLSConfig(s contextstore.Store) (*tls.Config, error) {
	tlsData, err := c.LoadTLSData(s)
	if err != nil {
		return nil, err
	}
	if tlsData == nil && !c.SkipTLSVerify {
		// there is no specific tls config
		return nil, nil
	}
	var tlsOpts []func(*tls.Config)
	if tlsData != nil && tlsData.CA != nil {
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(tlsData.CA) {
			return nil, errors.New("failed to retrieve context tls info: ca.pem seems invalid")
		}
		tlsOpts = append(tlsOpts, func(cfg *tls.Config) {
			cfg.RootCAs = certPool
		})
	}
	if tlsData != nil && tlsData.Key != nil && tlsData.Cert != nil {
		x509cert, err := tls.X509KeyPair(tlsData.Cert, tlsData.Key)
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve context tls info")
		}
		tlsOpts = append(tlsOpts, func(cfg *tls.Config) {
			cfg.Certificates = []tls.Certificate{x509cert}
		})
	}
	if c.SkipTLSVerify {
		tlsOpts = append(tlsOpts, func(cfg *tls.Config) {
			cfg.InsecureSkipVerify = true
		})
	}
	return tlsconfig.ClientDefault(tlsOpts...), nil
}

func getMetaString(meta map[string]interface{}, key string) string {
	v, ok := meta[key]
	if !ok {
		return ""
	}
	r, _ := v.(string)
	return r
}

func getMetaBool(meta map[string]interface{}, key string) bool {
	v, ok := meta[key]
	if !ok {
		return false
	}
	r, _ := v.(bool)
	return r
}

// Parse parses a context docker endpoint metadata into a typed Context structure
func Parse(name string, metadata contextstore.ContextMetadata) (*Context, error) {
	ep, ok := metadata.Endpoints[dockerEndpointKey]
	if !ok {
		return nil, errors.New("cannot find docker endpoint in context")
	}
	host := getMetaString(ep, hostKey)
	skipTLSVerify := getMetaBool(ep, skipTLSVerifyKey)
	apiVersion := getMetaString(ep, apiVersionKey)
	return &Context{
		Name:          name,
		Host:          host,
		SkipTLSVerify: skipTLSVerify,
		APIVersion:    apiVersion,
	}, nil
}
