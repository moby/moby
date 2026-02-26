package containerd

import (
	"cmp"
	"context"
	"encoding/json"
	"net"
	"net/url"
	"slices"
	"sort"
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
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

const (
	imageIdentityCacheTTL      = 48 * time.Hour
	imageIdentityErrorCacheTTL = 15 * time.Minute
	imageIdentityWarmupTimeout = 2 * time.Minute

	imageIdentityRefreshConcurrency = 4
	imageIdentityRefreshTimeout     = 2 * time.Minute
	imageIdentityRefreshInterval    = 30 * time.Minute
	imageIdentityRefreshAhead       = imageIdentityRefreshInterval + imageIdentityRefreshTimeout
)

type imageIdentityCacheEntry = identitycache.Entry

type imageIdentityState struct {
	flight     singleflight.Group
	cacheMu    sync.Mutex
	cache      map[string]imageIdentityCacheEntry
	cacheStore identitycache.Backend

	stopRefreshOnce sync.Once
	stopRefresh     chan struct{}
	refreshDone     chan struct{}
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
			computedSignature = i.cacheComputedSignatureIdentity(ctx, cacheKey, desc, computedSignature, hasTransientVerificationError)
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

func (i *ImageService) cacheComputedSignatureIdentity(ctx context.Context, cacheKey string, desc ocispec.Descriptor, computedSignature *imagetypes.SignatureIdentity, hasTransientVerificationError bool) *imagetypes.SignatureIdentity {
	ttl := imageIdentityCacheTTL
	if hasTransientVerificationError {
		// signature verification errors can be temporary (e.g. no network),
		// so cache these for a shorter period
		ttl = imageIdentityErrorCacheTTL
	}
	if err := i.updateImageIdentityCache(ctx, cacheKey, computedSignature, ttl); err != nil {
		log.G(ctx).WithError(err).WithField("image", desc.Digest).Debug("failed to update image identity cache entry")
	}
	return computedSignature
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

func imageIdentityCacheEntryNeedsRefresh(entry imageIdentityCacheEntry, now time.Time) bool {
	if entry.ExpiresAt.IsZero() {
		return false
	}
	return !entry.ExpiresAt.After(now.Add(imageIdentityRefreshAhead))
}

func (i *ImageService) imageIdentityCacheKeysToRefresh(ctx context.Context, now time.Time) ([]string, error) {
	seen := map[string]struct{}{}

	i.identity.cacheMu.Lock()
	for key, entry := range i.identity.cache {
		if imageIdentityCacheEntryNeedsRefresh(entry, now) {
			seen[key] = struct{}{}
		}
	}
	i.identity.cacheMu.Unlock()

	if i.identity.cacheStore != nil {
		if err := i.identity.cacheStore.Walk(ctx, now, func(cacheKey string, entry imageIdentityCacheEntry) error {
			if imageIdentityCacheEntryNeedsRefresh(entry, now) {
				seen[cacheKey] = struct{}{}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	out := make([]string, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	sort.Strings(out)
	return out, nil
}

func (i *ImageService) refreshImageIdentityCache(ctx context.Context, now time.Time) {
	keys, err := i.imageIdentityCacheKeysToRefresh(ctx, now)
	if err != nil {
		log.G(ctx).WithError(err).Debug("failed to list image identity cache refresh candidates")
		return
	}
	if len(keys) == 0 {
		return
	}

	var g errgroup.Group
	g.SetLimit(imageIdentityRefreshConcurrency)
	for _, key := range keys {
		g.Go(func() error {
			if err := i.refreshImageIdentityCacheKey(ctx, key); err != nil {
				log.G(ctx).WithError(err).WithField("cacheKey", key).Debug("failed to refresh image identity cache entry")
			}
			return nil
		})
	}
	_ = g.Wait()
}

func (i *ImageService) pruneImageIdentityInMemoryCacheEntries(now time.Time) {
	i.identity.cacheMu.Lock()
	pruneImageIdentityCacheEntries(i.identity.cache, now)
	i.identity.cacheMu.Unlock()
}

func (i *ImageService) pruneImageIdentityCacheStore(ctx context.Context, now time.Time) {
	if i.identity.cacheStore == nil {
		return
	}
	if err := i.identity.cacheStore.PruneExpired(ctx, now); err != nil {
		log.G(ctx).WithError(err).Debug("failed to prune expired image identity cache entries")
	}
}

func (i *ImageService) runImageIdentityCacheMaintenance(ctx context.Context, now time.Time) {
	i.refreshImageIdentityCache(ctx, now)
	i.pruneImageIdentityInMemoryCacheEntries(now)
	i.pruneImageIdentityCacheStore(ctx, now)
}

func (i *ImageService) refreshImageIdentityCacheKey(ctx context.Context, cacheKey string) error {
	imageDigest, bestDigest, bestPlatform, err := parseImageIdentityCacheKey(cacheKey)
	if err != nil {
		return err
	}

	imgs, err := i.images.List(ctx, "target.digest=="+imageDigest.String())
	if err != nil {
		return err
	}
	if len(imgs) == 0 {
		return nil
	}

	platformMatcher, err := imageIdentityPlatformMatcher(bestPlatform)
	if err != nil {
		return err
	}

	for _, img := range imgs {
		multi, err := i.multiPlatformSummary(ctx, img, platformMatcher)
		if err != nil {
			continue
		}
		if multi.Best == nil {
			continue
		}
		if bestDigest != "" && multi.Best.Target().Digest != bestDigest {
			continue
		}
		computedSignature, hasTransientVerificationError := i.computeSignatureIdentity(ctx, img.Target, multi)
		_ = i.cacheComputedSignatureIdentity(ctx, cacheKey, img.Target, computedSignature, hasTransientVerificationError)
		return nil
	}

	return nil
}

func imageIdentityPlatformMatcher(platform string) (platforms.MatchComparer, error) {
	if platform == "" {
		return matchAnyWithPreference(platforms.Default(), nil), nil
	}
	parsed, err := platforms.Parse(platform)
	if err != nil {
		return nil, err
	}
	return platforms.Only(parsed), nil
}

func parseImageIdentityCacheKey(cacheKey string) (digest.Digest, digest.Digest, string, error) {
	parts := strings.Split(cacheKey, "|")
	if len(parts) != 3 {
		return "", "", "", errors.Errorf("invalid image identity cache key %q", cacheKey)
	}

	imageDigest, err := digest.Parse(parts[0])
	if err != nil {
		return "", "", "", errors.Wrapf(err, "invalid image digest in image identity cache key %q", cacheKey)
	}

	var bestDigest digest.Digest
	if parts[1] != "" {
		bestDigest, err = digest.Parse(parts[1])
		if err != nil {
			return "", "", "", errors.Wrapf(err, "invalid best digest in image identity cache key %q", cacheKey)
		}
	}

	return imageDigest, bestDigest, parts[2], nil
}

func (i *ImageService) startImageIdentityCacheRefresh() {
	i.identity.cacheMu.Lock()
	if i.identity.stopRefresh != nil {
		i.identity.cacheMu.Unlock()
		return
	}
	i.identity.stopRefresh = make(chan struct{})
	i.identity.refreshDone = make(chan struct{})
	stopCh := i.identity.stopRefresh
	doneCh := i.identity.refreshDone
	i.identity.cacheMu.Unlock()

	go func() {
		defer close(doneCh)
		runMaintenance := func() {
			now := time.Now()

			maintenanceCtx, cancel := context.WithTimeout(context.Background(), imageIdentityRefreshTimeout)
			defer cancel()
			i.runImageIdentityCacheMaintenance(maintenanceCtx, now)
		}

		// Run one pass right after startup to refresh expired entries and prune stale ones.
		runMaintenance()

		timer := time.NewTicker(imageIdentityRefreshInterval)
		defer timer.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-timer.C:
				runMaintenance()
			}
		}
	}()
}

func (i *ImageService) stopImageIdentityCacheRefresh() {
	i.identity.stopRefreshOnce.Do(func() {
		i.identity.cacheMu.Lock()
		stopCh := i.identity.stopRefresh
		doneCh := i.identity.refreshDone
		i.identity.cacheMu.Unlock()
		if stopCh != nil {
			close(stopCh)
		}
		if doneCh != nil {
			<-doneCh
		}
	})
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
		// Warm signature identities for all locally available image manifests.
		for _, ref := range multi.manifestIdentityRefs {
			if _, err := i.imageIdentity(warmCtx, img.Target, &multiPlatformSummary{
				Best:         ref.manifest,
				BestPlatform: ref.platform,
			}); err != nil {
				log.G(warmCtx).WithError(err).WithField("image", img.Name).Debug("failed to build image identity cache in background")
			}
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
