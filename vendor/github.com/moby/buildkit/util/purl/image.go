package purl

import (
	"strings"

	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	packageurl "github.com/package-url/packageurl-go"
	"github.com/pkg/errors"
)

// RefToPURL converts an image reference with optional platform constraint to a package URL.
// Image references are defined in https://github.com/distribution/distribution/blob/v2.8.1/reference/reference.go#L1
// Package URLs are defined in https://github.com/package-url/purl-spec
func RefToPURL(ref string, platform *ocispecs.Platform) (string, error) {
	named, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse ref %q", ref)
	}
	var qualifiers []packageurl.Qualifier

	if canonical, ok := named.(reference.Canonical); ok {
		qualifiers = append(qualifiers, packageurl.Qualifier{
			Key:   "digest",
			Value: canonical.Digest().String(),
		})
	} else {
		named = reference.TagNameOnly(named)
	}

	version := ""
	if tagged, ok := named.(reference.Tagged); ok {
		version = tagged.Tag()
	}

	name := reference.FamiliarName(named)

	ns := ""
	parts := strings.Split(name, "/")
	if len(parts) > 1 {
		ns = strings.Join(parts[:len(parts)-1], "/")
	}
	name = parts[len(parts)-1]

	if platform != nil {
		p := platforms.Normalize(*platform)
		qualifiers = append(qualifiers, packageurl.Qualifier{
			Key:   "platform",
			Value: platforms.Format(p),
		})
	}

	p := packageurl.NewPackageURL("docker", ns, name, version, qualifiers, "")
	return p.ToString(), nil
}

// PURLToRef converts a package URL to an image reference and platform.
func PURLToRef(purl string) (string, *ocispecs.Platform, error) {
	p, err := packageurl.FromString(purl)
	if err != nil {
		return "", nil, err
	}
	if p.Type != "docker" {
		return "", nil, errors.Errorf("invalid package type %q, expecting docker", p.Type)
	}
	ref := p.Name
	if p.Namespace != "" {
		ref = p.Namespace + "/" + ref
	}
	dgstVersion := ""
	if p.Version != "" {
		dgst, err := digest.Parse(p.Version)
		if err == nil {
			ref = ref + "@" + dgst.String()
			dgstVersion = dgst.String()
		} else {
			ref += ":" + p.Version
		}
	}
	var platform *ocispecs.Platform
	for _, q := range p.Qualifiers {
		if q.Key == "platform" {
			p, err := platforms.Parse(q.Value)
			if err != nil {
				return "", nil, err
			}
			platform = &p
		}
		if q.Key == "digest" {
			if dgstVersion != "" {
				if dgstVersion != q.Value {
					return "", nil, errors.Errorf("digest %q does not match version %q", q.Value, dgstVersion)
				}
				continue
			}
			dgst, err := digest.Parse(q.Value)
			if err != nil {
				return "", nil, err
			}
			ref = ref + "@" + dgst.String()
			dgstVersion = dgst.String()
		}
	}

	if dgstVersion == "" && p.Version == "" {
		ref += ":latest"
	}

	named, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return "", nil, errors.Wrapf(err, "invalid image url %q", purl)
	}

	return named.String(), platform, nil
}
