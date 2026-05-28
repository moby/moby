package attestation

import (
	"bytes"

	"github.com/moby/buildkit/exporter"
)

func Filter(attestations []exporter.Attestation, include map[string][]byte, exclude map[string][]byte) []exporter.Attestation {
	if len(include) == 0 && len(exclude) == 0 {
		return attestations
	}

	result := []exporter.Attestation{}
	for _, att := range attestations {
		meta := att.Metadata
		if meta == nil {
			meta = map[string][]byte{}
		}

		match := true
		for k, v := range include {
			if !bytes.Equal(meta[k], v) {
				match = false
				break
			}
		}
		if !match {
			continue
		}

		for k, v := range exclude {
			if bytes.Equal(meta[k], v) {
				match = false
				break
			}
		}
		if !match {
			continue
		}

		result = append(result, att)
	}
	return result
}
