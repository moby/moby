package ghatypes

import "github.com/sigstore/sigstore-go/pkg/fulcio/certificate"

type CacheConfig struct {
	Sign   *SignConfig  `toml:"sign"`
	Verify VerifyConfig `toml:"verify"`
}

type SignConfig struct {
	Command []string `toml:"command"`
}

type VerifyConfig struct {
	Required bool         `toml:"required"`
	Policy   VerifyPolicy `toml:"policy"`
}

type VerifyPolicy struct {
	TimestampThreshold int `toml:"timestampThreshold"`
	TlogThreshold      int `toml:"tlogThreshold"`
	certificate.Summary
}
