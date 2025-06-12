package attestations

import (
	"strings"

	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	"github.com/pkg/errors"
	"github.com/tonistiigi/go-csvvalue"
)

const (
	KeyTypeSbom       = "sbom"
	KeyTypeProvenance = "provenance"
)

const (
	defaultSBOMGenerator = "docker/buildkit-syft-scanner:stable-1"
	defaultSLSAVersion   = string(provenancetypes.ProvenanceSLSA02)
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
		if after, ok := strings.CutPrefix(k, "attest:"); ok {
			attests[strings.ToLower(after)] = v
			continue
		}
		if after, ok := strings.CutPrefix(k, "build-arg:BUILDKIT_ATTEST_"); ok {
			attests[strings.ToLower(after)] = v
			continue
		}
	}

	out := make(map[string]map[string]string)
	for k, v := range attests {
		attrs := make(map[string]string)
		out[k] = attrs
		switch k {
		case KeyTypeSbom:
			attrs["generator"] = defaultSBOMGenerator
		case KeyTypeProvenance:
			attrs["version"] = defaultSLSAVersion
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
