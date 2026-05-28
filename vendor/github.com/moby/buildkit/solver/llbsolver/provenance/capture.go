package provenance

import (
	"cmp"
	"maps"
	"slices"

	distreference "github.com/distribution/reference"
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	"github.com/moby/buildkit/solver/result"
	"github.com/moby/buildkit/util/urlutil"
	digest "github.com/opencontainers/go-digest"
)

type Result = result.Result[*Capture]

type Capture struct {
	Request             provenancetypes.Parameters
	Sources             provenancetypes.Sources
	NetworkAccess       bool
	IncompleteMaterials bool
	Samples             map[digest.Digest]*resourcestypes.Samples
}

func (c *Capture) Clone() *Capture {
	if c == nil {
		return nil
	}
	out := &Capture{
		NetworkAccess:       c.NetworkAccess,
		IncompleteMaterials: c.IncompleteMaterials,
	}
	if req := c.Request.Clone(); req != nil {
		out.Request = *req
	}
	out.Sources.Images = append(out.Sources.Images, c.Sources.Images...)
	out.Sources.ImageBlobs = append(out.Sources.ImageBlobs, c.Sources.ImageBlobs...)
	out.Sources.Local = append(out.Sources.Local, c.Sources.Local...)
	out.Sources.Git = append(out.Sources.Git, c.Sources.Git...)
	out.Sources.HTTP = append(out.Sources.HTTP, c.Sources.HTTP...)
	if len(c.Samples) > 0 {
		out.Samples = make(map[digest.Digest]*resourcestypes.Samples, len(c.Samples))
		maps.Copy(out.Samples, c.Samples)
	}
	return out
}

func (c *Capture) Merge(c2 *Capture) error {
	if c2 == nil {
		return nil
	}
	for _, i := range c2.Sources.Images {
		c.AddImage(i)
	}
	for _, i := range c2.Sources.ImageBlobs {
		c.AddImageBlob(i)
	}
	for _, l := range c2.Sources.Local {
		c.AddLocal(l)
	}
	for _, g := range c2.Sources.Git {
		c.AddGit(g)
	}
	for _, h := range c2.Sources.HTTP {
		c.AddHTTP(h)
	}
	for _, s := range c2.Request.Secrets {
		if s != nil {
			c.AddSecret(*s)
		}
	}
	for _, s := range c2.Request.SSH {
		if s != nil {
			c.AddSSH(*s)
		}
	}
	if c2.NetworkAccess {
		c.NetworkAccess = true
	}
	if c2.IncompleteMaterials {
		c.IncompleteMaterials = true
	}
	return nil
}

func (c *Capture) Sort() {
	slices.SortFunc(c.Sources.Images, func(a, b provenancetypes.ImageSource) int {
		return cmp.Compare(a.Ref, b.Ref)
	})
	slices.SortFunc(c.Sources.ImageBlobs, func(a, b provenancetypes.ImageBlobSource) int {
		return cmp.Compare(a.Ref, b.Ref)
	})
	slices.SortFunc(c.Sources.Local, func(a, b provenancetypes.LocalSource) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(c.Sources.Git, func(a, b provenancetypes.GitSource) int {
		return cmp.Compare(a.URL, b.URL)
	})
	slices.SortFunc(c.Sources.HTTP, func(a, b provenancetypes.HTTPSource) int {
		return cmp.Compare(a.URL, b.URL)
	})
	slices.SortFunc(c.Request.Secrets, func(a, b *provenancetypes.Secret) int {
		return cmp.Compare(a.ID, b.ID)
	})
	slices.SortFunc(c.Request.SSH, func(a, b *provenancetypes.SSH) int {
		return cmp.Compare(a.ID, b.ID)
	})
}

// OptimizeImageSources filters out image sources by digest reference if same digest
// is already present by a tag reference.
func (c *Capture) OptimizeImageSources() error {
	m := map[string]struct{}{}
	for _, i := range c.Sources.Images {
		ref, nameTag, err := parseRefName(i.Ref)
		if err != nil {
			return err
		}
		if _, ok := ref.(distreference.Canonical); !ok {
			m[nameTag] = struct{}{}
		}
	}

	images := make([]provenancetypes.ImageSource, 0, len(c.Sources.Images))
	for _, i := range c.Sources.Images {
		ref, nameTag, err := parseRefName(i.Ref)
		if err != nil {
			return err
		}
		if _, ok := ref.(distreference.Canonical); ok {
			if _, ok := m[nameTag]; ok {
				continue
			}
		}
		images = append(images, i)
	}
	c.Sources.Images = images
	return nil
}

func (c *Capture) AddImage(i provenancetypes.ImageSource) {
	for _, v := range c.Sources.Images {
		if v.Ref == i.Ref && v.Local == i.Local {
			if v.Platform == i.Platform {
				return
			}
			if v.Platform != nil && i.Platform != nil {
				// NOTE: Deliberately excluding OSFeatures, as there's no extant (or rational) case where a source image is an index and contains images distinguished only by OSFeature
				// See https://github.com/moby/buildkit/pull/4387#discussion_r1376234241 and https://github.com/opencontainers/image-spec/issues/1147
				if v.Platform.Architecture == i.Platform.Architecture && v.Platform.OS == i.Platform.OS && v.Platform.OSVersion == i.Platform.OSVersion && v.Platform.Variant == i.Platform.Variant {
					return
				}
			}
		}
	}
	c.Sources.Images = append(c.Sources.Images, i)
}

func (c *Capture) AddImageBlob(i provenancetypes.ImageBlobSource) {
	for _, v := range c.Sources.ImageBlobs {
		if v.Ref == i.Ref && v.Local == i.Local {
			return
		}
	}
	c.Sources.ImageBlobs = append(c.Sources.ImageBlobs, i)
}

func (c *Capture) AddLocal(l provenancetypes.LocalSource) {
	for _, v := range c.Sources.Local {
		if v.Name == l.Name {
			return
		}
	}
	c.Sources.Local = append(c.Sources.Local, l)
}

func (c *Capture) AddGit(g provenancetypes.GitSource) {
	g.URL = urlutil.RedactCredentials(g.URL)
	// Dedupe on the tuple (URL, Bundle.URL). Two records with the same
	// URL but different bundle identity (e.g. the same repo referenced
	// once normally and once through a bundle, or through two different
	// bundle locators) must both be preserved so neither material is
	// silently dropped from the provenance. Bundle.URL is the canonical
	// bundle identity: since scheme/ref/digest are derived from it,
	// different URLs imply different bundle identity.
	for _, v := range c.Sources.Git {
		if v.URL != g.URL {
			continue
		}
		if bundleKey(v.Bundle) == bundleKey(g.Bundle) {
			return
		}
	}
	c.Sources.Git = append(c.Sources.Git, g)
}

// bundleKey returns a comparable identity for dedupe. Nil bundles collapse
// to the empty key.
func bundleKey(b *provenancetypes.GitBundle) string {
	if b == nil {
		return ""
	}
	return b.URL
}

func (c *Capture) AddHTTP(h provenancetypes.HTTPSource) {
	h.URL = urlutil.RedactCredentials(h.URL)
	for _, v := range c.Sources.HTTP {
		if v.URL == h.URL {
			return
		}
	}
	c.Sources.HTTP = append(c.Sources.HTTP, h)
}

func (c *Capture) AddSecret(s provenancetypes.Secret) {
	for i, v := range c.Request.Secrets {
		if v.ID == s.ID {
			if !s.Optional {
				c.Request.Secrets[i].Optional = false
			}
			return
		}
	}
	c.Request.Secrets = append(c.Request.Secrets, &s)
}

func (c *Capture) AddSSH(s provenancetypes.SSH) {
	if s.ID == "" {
		s.ID = "default"
	}
	for i, v := range c.Request.SSH {
		if v.ID == s.ID {
			if !s.Optional {
				c.Request.SSH[i].Optional = false
			}
			return
		}
	}
	c.Request.SSH = append(c.Request.SSH, &s)
}

func (c *Capture) AddSamples(dgst digest.Digest, samples *resourcestypes.Samples) {
	if c.Samples == nil {
		c.Samples = map[digest.Digest]*resourcestypes.Samples{}
	}
	c.Samples[dgst] = samples
}

func parseRefName(s string) (distreference.Named, string, error) {
	ref, err := distreference.ParseNormalizedNamed(s)
	if err != nil {
		return nil, "", err
	}
	name := ref.Name()
	tag := "latest"
	if r, ok := ref.(distreference.Tagged); ok {
		tag = r.Tag()
	}
	return ref, name + ":" + tag, nil
}
