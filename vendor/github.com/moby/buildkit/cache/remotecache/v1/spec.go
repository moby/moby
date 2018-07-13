package cacheimport

import (
	"time"

	digest "github.com/opencontainers/go-digest"
)

const CacheConfigMediaTypeV0 = "application/vnd.buildkit.cacheconfig.v0"

type CacheConfig struct {
	Layers  []CacheLayer  `json:"layers,omitempty"`
	Records []CacheRecord `json:"records,omitempty"`
}

type CacheLayer struct {
	Blob        digest.Digest `json:"blob,omitempty"`
	ParentIndex int           `json:"parent,omitempty"`
}

type CacheRecord struct {
	Results []CacheResult  `json:"layers,omitempty"`
	Digest  digest.Digest  `json:"digest,omitempty"`
	Inputs  [][]CacheInput `json:"inputs,omitempty"`
}

type CacheResult struct {
	LayerIndex int       `json:"layer"`
	CreatedAt  time.Time `json:"createdAt,omitempty"`
}

type CacheInput struct {
	Selector  string `json:"selector,omitempty"`
	LinkIndex int    `json:"link"`
}
