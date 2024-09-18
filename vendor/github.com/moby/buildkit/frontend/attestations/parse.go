package attestations

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/tonistiigi/go-csvvalue"
)

const (
	KeyTypeSbom       = "sbom"
	KeyTypeProvenance = "provenance"
)

const (
	defaultSBOMGenerator = "docker/buildkit-syft-scanner:stable-1"
)

func Filter(v map[string]string) map[string]string {
	attests := make(map[string]string)
	for k, v := range v {
		if strings.HasPrefix(k, "attest:") {
			attests[k] = v
			continue
		}
		if strings.HasPrefix(k, "build-arg:BUILDKIT_ATTEST_") {
			attests[k] = v
			continue
		}
	}
	return attests
}

func Validate(values map[string]map[string]string) (map[string]map[string]string, error) {
	for k := range values {
		if k != KeyTypeSbom && k != KeyTypeProvenance {
			return nil, errors.Errorf("unknown attestation type %q", k)
		}
	}
	return values, nil
}

func Parse(values map[string]string) (map[string]map[string]string, error) {
	attests := make(map[string]string)
	for k, v := range values {
		if strings.HasPrefix(k, "attest:") {
			attests[strings.ToLower(strings.TrimPrefix(k, "attest:"))] = v
			continue
		}
		if strings.HasPrefix(k, "build-arg:BUILDKIT_ATTEST_") {
			attests[strings.ToLower(strings.TrimPrefix(k, "build-arg:BUILDKIT_ATTEST_"))] = v
			continue
		}
	}

	out := make(map[string]map[string]string)
	for k, v := range attests {
		attrs := make(map[string]string)
		out[k] = attrs
		if k == KeyTypeSbom {
			attrs["generator"] = defaultSBOMGenerator
		}
		if v == "" {
			continue
		}
		fields, err := csvvalue.Fields(v, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", k)
		}
		for _, field := range fields {
			parts := strings.SplitN(field, "=", 2)
			if len(parts) != 2 {
				parts = append(parts, "")
			}
			attrs[parts[0]] = parts[1]
		}
	}

	return Validate(out)
}
