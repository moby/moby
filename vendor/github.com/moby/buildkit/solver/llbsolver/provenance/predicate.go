package provenance

import (
	"maps"
	"strings"

	"github.com/containerd/platforms"
	slsa "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
	slsa1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/moby/buildkit/frontend/dockerfile/dfgitutil"
	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	"github.com/moby/buildkit/util/purl"
	"github.com/moby/buildkit/util/urlutil"
	"github.com/package-url/packageurl-go"
)

func slsaMaterials(srcs provenancetypes.Sources) ([]slsa.ProvenanceMaterial, error) {
	count := len(srcs.Images) + len(srcs.ImageBlobs) + len(srcs.Git) + len(srcs.HTTP)
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

	for _, s := range srcs.ImageBlobs {
		purlType := packageurl.TypeDocker
		if s.Local {
			purlType = packageurl.TypeOCI
		}
		uri, err := purl.RefToPURL(purlType, s.Ref, nil)
		if err != nil {
			return nil, err
		}
		uri, err = setPURLQualifier(uri, packageurl.Qualifier{
			Key:   "ref_type",
			Value: "blob",
		})
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
			URI:    s.URL,
			Digest: digestSetForCommit(s.Commit),
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

func digestSetForCommit(commit string) slsa.DigestSet {
	dset := slsa.DigestSet{}
	if len(commit) == 64 {
		dset["sha256"] = commit
	} else {
		dset["sha1"] = commit
	}
	return dset
}

func setPURLQualifier(uri string, q packageurl.Qualifier) (string, error) {
	p, err := packageurl.FromString(uri)
	if err != nil {
		return "", err
	}
	for i, qq := range p.Qualifiers {
		if qq.Key == q.Key {
			p.Qualifiers[i].Value = q.Value
			return p.ToString(), nil
		}
	}
	p.Qualifiers = append(p.Qualifiers, q)
	return p.ToString(), nil
}

func findMaterial(srcs provenancetypes.Sources, uri string) (*slsa.ProvenanceMaterial, bool) {
	uri, _ = dfgitutil.FragmentFormat(uri)
	for _, s := range srcs.Git {
		if s.URL == uri {
			return &slsa.ProvenanceMaterial{
				URI:    s.URL,
				Digest: digestSetForCommit(s.Commit),
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

func NewPredicate(c *Capture) (*provenancetypes.ProvenancePredicateSLSA1, error) {
	materials, err := slsaMaterials(c.Sources)
	if err != nil {
		return nil, err
	}
	var resolvedDeps []slsa1.ResourceDescriptor
	for _, m := range materials {
		resolvedDeps = append(resolvedDeps, slsa1.ResourceDescriptor{
			URI:    m.URI,
			Digest: m.Digest,
		})
	}

	args := maps.Clone(c.Args)

	contextKey := "context"
	if v, ok := args["contextkey"]; ok && v != "" {
		contextKey = v
	} else if v, ok := c.Args["input:context"]; ok && v != "" {
		contextKey = "input:context"
	}

	ext := provenancetypes.ProvenanceExternalParametersSLSA1{}
	if v, ok := args[contextKey]; ok && v != "" {
		if m, ok := findMaterial(c.Sources, v); ok {
			ext.ConfigSource.URI = m.URI
			ext.ConfigSource.Digest = m.Digest
		} else {
			ext.ConfigSource.URI = v
		}
		ext.ConfigSource.URI = urlutil.RedactCredentials(ext.ConfigSource.URI)
		delete(args, contextKey)
	}

	if v, ok := args["filename"]; ok && v != "" {
		ext.ConfigSource.Path = v
		delete(args, "filename")
	}

	vcs := make(map[string]string)
	for k, v := range args {
		if strings.HasPrefix(k, "vcs:") {
			if k == "vcs:source" {
				v = urlutil.RedactCredentials(v)
			}
			delete(args, k)
			if v != "" {
				vcs[strings.TrimPrefix(k, "vcs:")] = v
			}
		}
	}

	internal := provenancetypes.ProvenanceInternalParametersSLSA1{}
	internal.BuilderPlatform = platforms.Format(platforms.Normalize(platforms.DefaultSpec()))

	req := provenancetypes.Parameters{}
	req.Frontend = c.Frontend
	req.Args = args
	for _, s := range c.Secrets {
		req.Secrets = append(req.Secrets, &provenancetypes.Secret{
			ID:       s.ID,
			Optional: s.Optional,
		})
	}
	for _, s := range c.SSH {
		req.SSH = append(req.SSH, &provenancetypes.SSH{
			ID:       s.ID,
			Optional: s.Optional,
		})
	}
	for _, s := range c.Sources.Local {
		req.Locals = append(req.Locals, &provenancetypes.LocalSource{
			Name: s.Name,
		})
	}
	ext.Request = req

	incompleteMaterials := c.IncompleteMaterials
	if !incompleteMaterials {
		if len(c.Sources.Local) > 0 {
			incompleteMaterials = true
		}
	}

	pr := &provenancetypes.ProvenancePredicateSLSA1{
		BuildDefinition: provenancetypes.ProvenanceBuildDefinitionSLSA1{
			ProvenanceBuildDefinition: slsa1.ProvenanceBuildDefinition{
				BuildType:            provenancetypes.BuildKitBuildType1,
				ResolvedDependencies: resolvedDeps,
			},
			ExternalParameters: ext,
			InternalParameters: internal,
		},
		RunDetails: provenancetypes.ProvenanceRunDetailsSLSA1{
			Metadata: &provenancetypes.ProvenanceMetadataSLSA1{
				Completeness: provenancetypes.BuildKitComplete{
					Request:              c.Frontend != "",
					ResolvedDependencies: !incompleteMaterials,
				},
				Hermetic: !incompleteMaterials && !c.NetworkAccess,
			},
		},
	}

	if len(vcs) > 0 {
		pr.RunDetails.Metadata.BuildKitMetadata.VCS = vcs
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
