package provenance

import (
	"maps"
	"slices"
	"strings"

	"github.com/containerd/platforms"
	slsa "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
	slsa1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/moby/buildkit/frontend/dockerfile/dfgitutil"
	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/purl"
	"github.com/moby/buildkit/util/urlutil"
	digest "github.com/opencontainers/go-digest"
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
		// Git material URI is always the raw repository URL (same shape
		// for bundle-backed and normal git sources).
		out = append(out, slsa.ProvenanceMaterial{
			URI:    s.URL,
			Digest: digestSetForCommit(s.Commit),
		})
		if s.Bundle == nil {
			continue
		}

		// Bundle-backed git source: parse the canonical locator into
		// its scheme / ref-body / digest components on demand. On parse
		// failure (shouldn't happen — validateBundleAttrs rejects bad
		// locators at LLB op construction time) skip bundle material
		// emission rather than erroring.
		scheme, refBody, bundleDgst := parseBundleLocatorURL(s.Bundle.URL)
		if scheme == "" || refBody == "" || bundleDgst == "" {
			continue
		}

		bundlePurlType := packageurl.TypeDocker
		if scheme == srctypes.OCIBlobScheme {
			bundlePurlType = packageurl.TypeOCI
		}
		bundleURI, err := purl.RefToPURL(bundlePurlType, refBody, nil)
		if err != nil {
			return nil, err
		}
		bundleURI, err = setPURLQualifier(bundleURI, packageurl.Qualifier{
			Key:   "ref_type",
			Value: "bundle",
		})
		if err != nil {
			return nil, err
		}
		bundleURI, err = setPURLQualifier(bundleURI, packageurl.Qualifier{
			Key:   "vcs_url",
			Value: s.URL,
		})
		if err != nil {
			return nil, err
		}

		out = append(out, slsa.ProvenanceMaterial{
			URI: bundleURI,
			Digest: slsa.DigestSet{
				bundleDgst.Algorithm().String(): bundleDgst.Hex(),
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

// parseBundleLocatorURL parses a "<scheme>://<ref>@<algo>:<hex>" bundle
// locator into its components. It returns empty values on parse failure; the
// caller handles that by falling back to the non-bundle emission path.
//
// The identifier package has a stricter parseBundleLocator used at LLB op
// construction time (validateBundleAttrs) that validates the reference and
// digest algorithm. We intentionally don't import that helper here because
// the identifier package imports this provenance package. Since the locator
// has already been validated upstream, the logic here just splits the
// locator back into its parts for purl emission.
func parseBundleLocatorURL(raw string) (scheme, refBody string, dgst digest.Digest) {
	const sep = "://"
	i := strings.Index(raw, sep)
	if i <= 0 {
		return "", "", ""
	}
	scheme = raw[:i]
	body := raw[i+len(sep):]
	at := strings.LastIndex(body, "@")
	if at <= 0 {
		return "", "", ""
	}
	return scheme, body[:at], digest.Digest(body[at+1:])
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
	configURI := uri
	if formatted, ok := dfgitutil.FragmentFormat(uri, true); ok {
		configURI = formatted
	}
	uri, _ = dfgitutil.FragmentFormat(uri, false)
	for _, s := range srcs.Git {
		if s.URL == uri {
			return &slsa.ProvenanceMaterial{
				URI:    configURI,
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

	ext := provenancetypes.ProvenanceExternalParametersSLSA1{}
	reqProv := RequestProvenance(c.Request.Frontend, maps.Clone(c.Request.Args), c.Sources)
	ext.ConfigSource = reqProv.ConfigSource

	vcs := make(map[string]string)
	req := c.Request.Clone()
	if req == nil {
		req = &provenancetypes.Parameters{}
	}
	if reqProv.Request != nil {
		req.Frontend = reqProv.Request.Frontend
		req.Args = reqProv.Request.Args
	}
	for k, v := range req.Args {
		if strings.HasPrefix(k, "vcs:") {
			if k == "vcs:source" {
				v = urlutil.RedactCredentials(v)
			}
			delete(req.Args, k)
			if v != "" {
				vcs[strings.TrimPrefix(k, "vcs:")] = v
			}
		}
	}

	internal := provenancetypes.ProvenanceInternalParametersSLSA1{}
	internal.BuilderPlatform = platforms.Format(platforms.Normalize(platforms.DefaultSpec()))

	for _, s := range c.Sources.Local {
		req.Locals = append(req.Locals, &provenancetypes.LocalSource{
			Name: s.Name,
		})
	}
	ext.Request = *req

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
					Request:              c.Request.Frontend != "",
					ResolvedDependencies: !incompleteMaterials,
				},
				Hermetic: !incompleteMaterials && !c.NetworkAccess,
			},
		},
	}

	if len(vcs) > 0 {
		pr.RunDetails.Metadata.BuildKitMetadata.VCS = vcs
	}
	if c.ProxyNetwork {
		pr.RunDetails.Metadata.BuildKitMetadata.Network = &provenancetypes.NetworkMetadata{
			Mode: "proxy",
		}
		if len(c.ProxyIncomplete) > 0 {
			pr.RunDetails.Metadata.BuildKitMetadata.Network.Proxy = &provenancetypes.ProxyNetworkMetadata{
				Incomplete: slices.Clone(c.ProxyIncomplete),
			}
		}
	}

	return pr, nil
}

func RequestProvenance(frontend string, args map[string]string, srcs provenancetypes.Sources) *provenancetypes.RequestProvenance {
	args = maps.Clone(args)
	contextKey := "context"
	if v, ok := args["contextkey"]; ok && v != "" {
		contextKey = v
	} else if v, ok := args["input:context"]; ok && v != "" {
		contextKey = "input:context"
	}

	ext := provenancetypes.RequestProvenance{
		Request: &provenancetypes.Parameters{
			Frontend: frontend,
			Args:     args,
		},
	}
	if v, ok := args[contextKey]; ok && v != "" {
		if m, ok := findMaterial(srcs, v); ok {
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
	if len(args) == 0 {
		ext.Request.Args = nil
	}
	return &ext
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
