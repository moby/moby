package containerd

import (
	"cmp"
	"context"
	"encoding/json"
	"net"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	c8dimages "github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/pkg/labels"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	imagetypes "github.com/moby/moby/api/types/image"
	"github.com/moby/moby/v2/daemon/containerd/identitycache"
	"github.com/moby/moby/v2/daemon/internal/builder-next/exporter"
	policyimage "github.com/moby/policy-helpers/image"
	policytypes "github.com/moby/policy-helpers/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
)

const (
	imageIdentityCacheTTL      = 48 * time.Hour
	imageIdentityErrorCacheTTL = 15 * time.Minute
	imageIdentityWarmupTimeout = 2 * time.Minute
)

type imageIdentityCacheEntry = identitycache.Entry

type imageIdentityState struct {
	flight     singleflight.Group
	cacheMu    sync.Mutex
	cache      map[string]imageIdentityCacheEntry
	cacheStore identitycache.Backend
}

func (i *ImageService) imageIdentity(ctx context.Context, desc ocispec.Descriptor, multi *multiPlatformSummary) (*imagetypes.Identity, error) {
	return i.imageIdentityWithCachePolicy(ctx, desc, multi, true)
}

func (i *ImageService) imageIdentityFromCache(ctx context.Context, desc ocispec.Descriptor, multi *multiPlatformSummary) (*imagetypes.Identity, error) {
	return i.imageIdentityWithCachePolicy(ctx, desc, multi, false)
}

func (i *ImageService) imageIdentityWithCachePolicy(ctx context.Context, desc ocispec.Descriptor, multi *multiPlatformSummary, computeOnCacheMiss bool) (*imagetypes.Identity, error) {
	info, err := i.content.Info(ctx, desc.Digest)
	if err != nil {
		return nil, err
	}

	identity := imageIdentityFromLabels(ctx, info.Labels)
	bestDigest, bestPlatform := imageIdentityBestMatch(multi)
	cacheKey := imageIdentityCacheKey(desc.Digest.String(), bestDigest, bestPlatform)
	signature, ok, err := i.imageSignatureIdentityFromCache(ctx, cacheKey)
	if err != nil {
		log.G(ctx).WithError(err).WithField("image", desc.Digest).Debug("failed to load image identity cache entry")
	}
	if !ok && computeOnCacheMiss {
		v, err, _ := i.identity.flight.Do(cacheKey, func() (any, error) {
			if cached, ok, err := i.imageSignatureIdentityFromCache(ctx, cacheKey); err == nil && ok {
				return cached, nil
			} else if err != nil {
				log.G(ctx).WithError(err).WithField("image", desc.Digest).Debug("failed to refresh image identity cache entry")
			}

			computedSignature, hasTransientVerificationError := i.computeSignatureIdentity(ctx, desc, multi)
			ttl := imageIdentityCacheTTL
			if hasTransientVerificationError {
				// signature verification errors can be temporary (e.g. no network),
				// so cache these for a shorter period
				ttl = imageIdentityErrorCacheTTL
			}
			if err := i.updateImageIdentityCache(ctx, cacheKey, computedSignature, ttl); err != nil {
				log.G(ctx).WithError(err).WithField("image", desc.Digest).Debug("failed to update image identity cache entry")
			}

			return computedSignature, nil
		})
		if err != nil {
			return nil, err
		}
		cachedSignature, ok := v.(*imagetypes.SignatureIdentity)
		if !ok {
			return nil, errors.Errorf("unexpected cached signature identity type %T", v)
		}
		signature = cachedSignature
	}

	if signature != nil {
		identity.Signature = append(identity.Signature, *signature)
	}

	if len(identity.Build) == 0 && len(identity.Pull) == 0 && len(identity.Signature) == 0 {
		return nil, nil
	}

	return identity, nil
}

func imageIdentityBestMatch(multi *multiPlatformSummary) (bestDigest string, bestPlatform string) {
	if multi == nil || multi.Best == nil {
		return "", ""
	}
	return multi.Best.Target().Digest.String(), platforms.FormatAll(multi.BestPlatform)
}

func imageIdentityFromLabels(ctx context.Context, labelsByDigest map[string]string) *imagetypes.Identity {
	identity := &imagetypes.Identity{}
	seenRepos := make(map[string]struct{})

	for k, v := range labelsByDigest {
		if ref, ok := strings.CutPrefix(k, exporter.BuildRefLabel); ok {
			var val exporter.BuildRefLabelValue
			if err := json.Unmarshal([]byte(v), &val); err == nil {
				var createdAt time.Time
				if val.CreatedAt != nil {
					createdAt = *val.CreatedAt
				}
				identity.Build = append(identity.Build, imagetypes.BuildIdentity{
					Ref:       ref,
					CreatedAt: createdAt,
				})
			}
		}
		if registry, ok := strings.CutPrefix(k, labels.LabelDistributionSource+"."); ok {
			for repo := range strings.SplitSeq(v, ",") {
				ref, err := reference.ParseNormalizedNamed(registry + "/" + repo)
				if err != nil {
					log.G(ctx).WithError(err).Error("failed to parse image name as reference")
					continue
				}
				name := ref.Name()
				if _, ok := seenRepos[name]; ok {
					continue
				}
				seenRepos[name] = struct{}{}
				identity.Pull = append(identity.Pull, imagetypes.PullIdentity{
					Repository: name,
				})
			}
		}
	}

	slices.SortFunc(identity.Build, func(a, b imagetypes.BuildIdentity) int {
		return cmp.Compare(a.Ref, b.Ref)
	})

	return identity
}

func (i *ImageService) computeSignatureIdentity(ctx context.Context, desc ocispec.Descriptor, multi *multiPlatformSummary) (*imagetypes.SignatureIdentity, bool) {
	if multi == nil || multi.Best == nil {
		return nil, false
	}

	signatureIdentity, err := i.signatureIdentity(ctx, desc, multi.Best, multi.BestPlatform)
	if err != nil {
		log.G(ctx).WithError(err).Error("failed to validate image signature")
		return nil, signatureVerificationErrorIsTransient(err)
	}

	// verification errors are represented as a payload field. Treat only
	// network-like errors as transient so deterministic verification failures
	// remain cached with the normal TTL
	hasTransientVerificationError := signatureIdentity != nil && signatureVerificationMessageIsTransient(signatureIdentity.Error)
	return signatureIdentity, hasTransientVerificationError
}

func signatureVerificationErrorIsTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return signatureVerificationErrorIsTransient(urlErr.Err)
	}

	return signatureVerificationMessageIsTransient(err.Error())
}

func signatureVerificationMessageIsTransient(msg string) bool {
	msg = strings.ToLower(msg)
	// TODO: replace message-based transient detection with structured error
	//  classification from policy-helpers / signature verification (e.g. typed
	//  retryable errors), so cache TTL decisions do not depend on string
	//  matching.
	for _, transient := range []string{
		"context deadline exceeded",
		"i/o timeout",
		"tls handshake timeout",
		"no such host",
		"temporary failure in name resolution",
		"connection refused",
		"connection reset by peer",
		"network is unreachable",
		"dial tcp",
	} {
		if strings.Contains(msg, transient) {
			return true
		}
	}
	return false
}

func imageIdentityCacheKey(imageDigest, bestDigest, bestPlatform string) string {
	return strings.Join([]string{imageDigest, bestDigest, bestPlatform}, "|")
}

func (i *ImageService) imageSignatureIdentityFromCache(ctx context.Context, cacheKey string) (*imagetypes.SignatureIdentity, bool, error) {
	now := time.Now()

	i.identity.cacheMu.Lock()
	if cached, ok := i.identity.cache[cacheKey]; ok {
		if now.After(cached.ExpiresAt) {
			delete(i.identity.cache, cacheKey)
		} else {
			i.identity.cacheMu.Unlock()
			return cloneSignatureIdentity(cached.Signature), true, nil
		}
	}
	i.identity.cacheMu.Unlock()

	if i.identity.cacheStore == nil {
		return nil, false, nil
	}
	cached, ok, err := i.identity.cacheStore.Load(ctx, cacheKey, now)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	i.identity.cacheMu.Lock()
	if i.identity.cache == nil {
		i.identity.cache = map[string]imageIdentityCacheEntry{}
	}
	i.identity.cache[cacheKey] = cached
	i.identity.cacheMu.Unlock()
	return cloneSignatureIdentity(cached.Signature), true, nil
}

func (i *ImageService) updateImageIdentityCache(ctx context.Context, cacheKey string, signature *imagetypes.SignatureIdentity, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}

	now := time.Now()
	entry := imageIdentityCacheEntry{
		CachedAt:  now,
		ExpiresAt: now.Add(ttl),
		Signature: cloneSignatureIdentity(signature),
	}

	i.identity.cacheMu.Lock()
	if i.identity.cache == nil {
		i.identity.cache = map[string]imageIdentityCacheEntry{}
	}
	i.identity.cache[cacheKey] = entry
	pruneImageIdentityCacheEntries(i.identity.cache, now)
	i.identity.cacheMu.Unlock()

	if i.identity.cacheStore == nil {
		return nil
	}
	return i.identity.cacheStore.Store(ctx, cacheKey, entry, now)
}

func cloneSignatureIdentity(s *imagetypes.SignatureIdentity) *imagetypes.SignatureIdentity {
	if s == nil {
		return nil
	}
	out := *s
	out.Timestamps = slices.Clone(s.Timestamps)
	out.Warnings = slices.Clone(s.Warnings)
	if s.Signer != nil {
		signer := *s.Signer
		out.Signer = &signer
	}
	return &out
}

func pruneImageIdentityCacheEntries(entries map[string]imageIdentityCacheEntry, now time.Time) {
	for key, entry := range entries {
		if now.After(entry.ExpiresAt) {
			delete(entries, key)
		}
	}
}

func (i *ImageService) warmImageIdentityCache(ctx context.Context, img c8dimages.Image) {
	if i.policyVerifier == nil {
		return
	}
	go func() {
		warmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), imageIdentityWarmupTimeout)
		defer cancel()
		multi, err := i.multiPlatformSummary(warmCtx, img, matchAnyWithPreference(platforms.Default(), nil))
		if err != nil {
			log.G(warmCtx).WithError(err).WithField("image", img.Name).Debug("failed to build image identity cache in background")
			return
		}
		if _, err := i.imageIdentity(warmCtx, img.Target, multi); err != nil {
			log.G(warmCtx).WithError(err).WithField("image", img.Name).Debug("failed to build image identity cache in background")
		}
	}()
}

func (i *ImageService) signatureIdentity(ctx context.Context, desc ocispec.Descriptor, img *ImageManifest, platform ocispec.Platform) (*imagetypes.SignatureIdentity, error) {
	rp := &referrersProvider{Store: i.content}
	sc, err := policyimage.ResolveSignatureChain(ctx, rp, desc, &platform)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving signature chain for image %s", desc.Digest)
	}
	if sc.SignatureManifest == nil || sc.ImageManifest == nil {
		return nil, nil
	}

	if sc.ImageManifest.Digest != img.Target().Digest {
		log.L.Infof("signature chain image manifest digest mismatch: %s != %s", sc.ImageManifest.Digest, img.Target().Digest)
		return nil, nil
	}

	if i.policyVerifier == nil {
		return nil, nil
	}

	v, err := i.policyVerifier()
	if err != nil {
		return nil, err
	}

	out := &imagetypes.SignatureIdentity{}

	si, err := v.VerifyImage(ctx, rp, desc, &platform)
	if err != nil {
		out.Error = err.Error()
		return out, nil
	}

	out.Name = si.Name()
	out.DockerReference = si.DockerReference

	if si.IsDHI { // update upstream to also use known signer type instead of bool
		out.KnownSigner = imagetypes.KnownSignerDHI
	}

	switch si.SignatureType {
	case policytypes.SignatureBundleV03:
		out.SignatureType = imagetypes.SignatureTypeBundleV03
	case policytypes.SignatureSimpleSigningV1:
		out.SignatureType = imagetypes.SignatureTypeSimpleSigningV1
	}

	for _, ts := range si.Timestamps {
		out.Timestamps = append(out.Timestamps, imagetypes.SignatureTimestamp{
			Type:      imagetypes.SignatureTimestampType(ts.Type),
			URI:       ts.URI,
			Timestamp: ts.Timestamp,
		})
	}

	if signer := si.Signer; signer != nil {
		out.Signer = &imagetypes.SignerIdentity{
			CertificateIssuer:                   signer.CertificateIssuer,
			SubjectAlternativeName:              signer.SubjectAlternativeName,
			Issuer:                              signer.Issuer,
			BuildSignerURI:                      signer.BuildSignerURI,
			BuildSignerDigest:                   signer.BuildSignerDigest,
			RunnerEnvironment:                   signer.RunnerEnvironment,
			SourceRepositoryURI:                 signer.SourceRepositoryURI,
			SourceRepositoryDigest:              signer.SourceRepositoryDigest,
			SourceRepositoryRef:                 signer.SourceRepositoryRef,
			SourceRepositoryIdentifier:          signer.SourceRepositoryIdentifier,
			SourceRepositoryOwnerURI:            signer.SourceRepositoryOwnerURI,
			SourceRepositoryOwnerIdentifier:     signer.SourceRepositoryOwnerIdentifier,
			BuildConfigURI:                      signer.BuildConfigURI,
			BuildConfigDigest:                   signer.BuildConfigDigest,
			BuildTrigger:                        signer.BuildTrigger,
			RunInvocationURI:                    signer.RunInvocationURI,
			SourceRepositoryVisibilityAtSigning: signer.SourceRepositoryVisibilityAtSigning,
		}
	}

	if si.TrustRootStatus.Error != "" {
		out.Warnings = append(out.Warnings, si.TrustRootStatus.Error)
	}

	return out, nil
}

type referrersProvider struct {
	content.Store
}

var _ policyimage.ReferrersProvider = &referrersProvider{}

func (p *referrersProvider) FetchReferrers(ctx context.Context, dgst digest.Digest, opts ...remotes.FetchReferrersOpt) ([]ocispec.Descriptor, error) {
	rfe := &referrersForExport{store: p.Store}
	descs, err := rfe.Referrers(ctx, ocispec.Descriptor{Digest: dgst})
	if err != nil {
		return nil, errors.Wrapf(err, "fetching referrers for %s", dgst)
	}

	if len(descs) == 0 {
		return nil, nil
	}

	cfg := &remotes.FetchReferrersConfig{}
	for _, opt := range opts {
		if err := opt(ctx, cfg); err != nil {
			return nil, err
		}
	}
	filter := make(map[string]struct{})
	for _, t := range cfg.ArtifactTypes {
		filter[t] = struct{}{}
	}

	if len(filter) == 0 {
		return descs, nil
	}

	out := make([]ocispec.Descriptor, 0, len(descs))
	for _, d := range descs {
		if _, ok := filter[d.MediaType]; ok {
			out = append(out, d)
		}
	}
	return out, nil
}
