package provenance

import (
	"strings"

	"github.com/containerd/containerd/platforms"
	slsa "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
	slsa02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	"github.com/moby/buildkit/util/purl"
	"github.com/moby/buildkit/util/urlutil"
	"github.com/package-url/packageurl-go"
)

func slsaMaterials(srcs provenancetypes.Sources) ([]slsa.ProvenanceMaterial, error) {
	count := len(srcs.Images) + len(srcs.Git) + len(srcs.HTTP)
	out := make([]slsa.ProvenanceMaterial, 0, count)

	for _, s := range srcs.Images {
		var uri string
		var err error
		if s.Local {
			uri, err = purl.RefToPURL(packageurl.TypeOCI, s.Ref, s.Platform)
		} else {
			uri, err = purl.RefToPURL(packageurl.TypeDocker, s.Ref, s.Platform)
		}
		if err != nil {
			return nil, err
		}
		material := slsa.ProvenanceMaterial{
			URI: uri,
		}
		if s.Digest != "" {
			material.Digest = slsa.DigestSet{
				s.Digest.Algorithm().String(): s.Digest.Hex(),
			}
		}
		out = append(out, material)
	}

	for _, s := range srcs.Git {
		out = append(out, slsa.ProvenanceMaterial{
			URI: s.URL,
			Digest: slsa.DigestSet{
				"sha1": s.Commit,
			},
		})
	}

	for _, s := range srcs.HTTP {
		out = append(out, slsa.ProvenanceMaterial{
			URI: s.URL,
			Digest: slsa.DigestSet{
				s.Digest.Algorithm().String(): s.Digest.Hex(),
			},
		})
	}

	return out, nil
}

func findMaterial(srcs provenancetypes.Sources, uri string) (*slsa.ProvenanceMaterial, bool) {
	for _, s := range srcs.Git {
		if s.URL == uri {
			return &slsa.ProvenanceMaterial{
				URI: s.URL,
				Digest: slsa.DigestSet{
					"sha1": s.Commit,
				},
			}, true
		}
	}
	for _, s := range srcs.HTTP {
		if s.URL == uri {
			return &slsa.ProvenanceMaterial{
				URI: s.URL,
				Digest: slsa.DigestSet{
					s.Digest.Algorithm().String(): s.Digest.Hex(),
				},
			}, true
		}
	}
	return nil, false
}

func NewPredicate(c *Capture) (*provenancetypes.ProvenancePredicate, error) {
	materials, err := slsaMaterials(c.Sources)
	if err != nil {
		return nil, err
	}
	inv := provenancetypes.ProvenanceInvocation{}

	contextKey := "context"
	if v, ok := c.Args["contextkey"]; ok && v != "" {
		contextKey = v
	}

	if v, ok := c.Args[contextKey]; ok && v != "" {
		if m, ok := findMaterial(c.Sources, v); ok {
			inv.ConfigSource.URI = m.URI
			inv.ConfigSource.Digest = m.Digest
		} else {
			inv.ConfigSource.URI = v
		}
		inv.ConfigSource.URI = urlutil.RedactCredentials(inv.ConfigSource.URI)
		delete(c.Args, contextKey)
	}

	if v, ok := c.Args["filename"]; ok && v != "" {
		inv.ConfigSource.EntryPoint = v
		delete(c.Args, "filename")
	}

	vcs := make(map[string]string)
	for k, v := range c.Args {
		if strings.HasPrefix(k, "vcs:") {
			if k == "vcs:source" {
				v = urlutil.RedactCredentials(v)
			}
			delete(c.Args, k)
			if v != "" {
				vcs[strings.TrimPrefix(k, "vcs:")] = v
			}
		}
	}

	inv.Environment.Platform = platforms.Format(platforms.Normalize(platforms.DefaultSpec()))

	inv.Parameters.Frontend = c.Frontend
	inv.Parameters.Args = c.Args

	for _, s := range c.Secrets {
		inv.Parameters.Secrets = append(inv.Parameters.Secrets, &provenancetypes.Secret{
			ID:       s.ID,
			Optional: s.Optional,
		})
	}
	for _, s := range c.SSH {
		inv.Parameters.SSH = append(inv.Parameters.SSH, &provenancetypes.SSH{
			ID:       s.ID,
			Optional: s.Optional,
		})
	}
	for _, s := range c.Sources.Local {
		inv.Parameters.Locals = append(inv.Parameters.Locals, &provenancetypes.LocalSource{
			Name: s.Name,
		})
	}

	incompleteMaterials := c.IncompleteMaterials
	if !incompleteMaterials {
		if len(c.Sources.Local) > 0 {
			incompleteMaterials = true
		}
	}

	pr := &provenancetypes.ProvenancePredicate{
		Invocation: inv,
		ProvenancePredicate: slsa02.ProvenancePredicate{
			BuildType: provenancetypes.BuildKitBuildType,
			Materials: materials,
		},
		Metadata: &provenancetypes.ProvenanceMetadata{
			ProvenanceMetadata: slsa02.ProvenanceMetadata{
				Completeness: slsa02.ProvenanceComplete{
					Parameters:  c.Frontend != "",
					Environment: true,
					Materials:   !incompleteMaterials,
				},
			},
			Hermetic: !incompleteMaterials && !c.NetworkAccess,
		},
	}

	if len(vcs) > 0 {
		pr.Metadata.BuildKitMetadata.VCS = vcs
	}

	return pr, nil
}

func FilterArgs(m map[string]string) map[string]string {
	var hostSpecificArgs = map[string]struct{}{
		"cgroup-parent":      {},
		"image-resolve-mode": {},
		"platform":           {},
		"cache-imports":      {},
	}
	const defaultContextKey = "context"
	contextKey := defaultContextKey
	if v, ok := m["contextkey"]; ok && v != "" {
		contextKey = v
	}
	out := make(map[string]string)
	for k, v := range m {
		if _, ok := hostSpecificArgs[k]; ok {
			continue
		}
		if strings.HasPrefix(k, "attest:") {
			continue
		}
		if k == contextKey || strings.HasPrefix(k, defaultContextKey+":") {
			v = urlutil.RedactCredentials(v)
		}
		out[k] = v
	}
	return out
}
