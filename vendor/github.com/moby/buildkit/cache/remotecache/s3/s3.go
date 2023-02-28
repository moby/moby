package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	aws_config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/containerd/containerd/content"
	"github.com/moby/buildkit/cache/remotecache"
	v1 "github.com/moby/buildkit/cache/remotecache/v1"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/util/progress"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	attrBucket          = "bucket"
	attrRegion          = "region"
	attrPrefix          = "prefix"
	attrManifestsPrefix = "manifests_prefix"
	attrBlobsPrefix     = "blobs_prefix"
	attrName            = "name"
	attrTouchRefresh    = "touch_refresh"
	attrEndpointURL     = "endpoint_url"
	attrAccessKeyID     = "access_key_id"
	attrSecretAccessKey = "secret_access_key"
	attrSessionToken    = "session_token"
	attrUsePathStyle    = "use_path_style"
)

type Config struct {
	Bucket          string
	Region          string
	Prefix          string
	ManifestsPrefix string
	BlobsPrefix     string
	Names           []string
	TouchRefresh    time.Duration
	EndpointURL     string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	UsePathStyle    bool
}

func getConfig(attrs map[string]string) (Config, error) {
	bucket, ok := attrs[attrBucket]
	if !ok {
		bucket, ok = os.LookupEnv("AWS_BUCKET")
		if !ok {
			return Config{}, errors.Errorf("bucket ($AWS_BUCKET) not set for s3 cache")
		}
	}

	region, ok := attrs[attrRegion]
	if !ok {
		region, ok = os.LookupEnv("AWS_REGION")
		if !ok {
			return Config{}, errors.Errorf("region ($AWS_REGION) not set for s3 cache")
		}
	}

	prefix := attrs[attrPrefix]

	manifestsPrefix, ok := attrs[attrManifestsPrefix]
	if !ok {
		manifestsPrefix = "manifests/"
	}

	blobsPrefix, ok := attrs[attrBlobsPrefix]
	if !ok {
		blobsPrefix = "blobs/"
	}

	names := []string{"buildkit"}
	name, ok := attrs[attrName]
	if ok {
		splittedNames := strings.Split(name, ";")
		if len(splittedNames) > 0 {
			names = splittedNames
		}
	}

	touchRefresh := 24 * time.Hour

	touchRefreshStr, ok := attrs[attrTouchRefresh]
	if ok {
		touchRefreshFromUser, err := time.ParseDuration(touchRefreshStr)
		if err == nil {
			touchRefresh = touchRefreshFromUser
		}
	}

	endpointURL := attrs[attrEndpointURL]
	accessKeyID := attrs[attrAccessKeyID]
	secretAccessKey := attrs[attrSecretAccessKey]
	sessionToken := attrs[attrSessionToken]

	usePathStyle := false
	usePathStyleStr, ok := attrs[attrUsePathStyle]
	if ok {
		usePathStyleUser, err := strconv.ParseBool(usePathStyleStr)
		if err == nil {
			usePathStyle = usePathStyleUser
		}
	}

	return Config{
		Bucket:          bucket,
		Region:          region,
		Prefix:          prefix,
		ManifestsPrefix: manifestsPrefix,
		BlobsPrefix:     blobsPrefix,
		Names:           names,
		TouchRefresh:    touchRefresh,
		EndpointURL:     endpointURL,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		SessionToken:    sessionToken,
		UsePathStyle:    usePathStyle,
	}, nil
}

// ResolveCacheExporterFunc for s3 cache exporter.
func ResolveCacheExporterFunc() remotecache.ResolveCacheExporterFunc {
	return func(ctx context.Context, g session.Group, attrs map[string]string) (remotecache.Exporter, error) {
		config, err := getConfig(attrs)
		if err != nil {
			return nil, err
		}

		s3Client, err := newS3Client(ctx, config)
		if err != nil {
			return nil, err
		}
		cc := v1.NewCacheChains()
		return &exporter{CacheExporterTarget: cc, chains: cc, s3Client: s3Client, config: config}, nil
	}
}

type exporter struct {
	solver.CacheExporterTarget
	chains   *v1.CacheChains
	s3Client *s3Client
	config   Config
}

func (*exporter) Name() string {
	return "exporting cache to s3"
}

func (e *exporter) Config() remotecache.Config {
	return remotecache.Config{
		Compression: compression.New(compression.Default),
	}
}

func (e *exporter) Finalize(ctx context.Context) (map[string]string, error) {
	cacheConfig, descs, err := e.chains.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	for i, l := range cacheConfig.Layers {
		dgstPair, ok := descs[l.Blob]
		if !ok {
			return nil, errors.Errorf("missing blob %s", l.Blob)
		}
		if dgstPair.Descriptor.Annotations == nil {
			return nil, errors.Errorf("invalid descriptor without annotations")
		}
		v, ok := dgstPair.Descriptor.Annotations["containerd.io/uncompressed"]
		if !ok {
			return nil, errors.Errorf("invalid descriptor without uncompressed annotation")
		}
		diffID, err := digest.Parse(v)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse uncompressed annotation")
		}

		key := e.s3Client.blobKey(dgstPair.Descriptor.Digest)
		exists, err := e.s3Client.exists(ctx, key)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to check file presence in cache")
		}
		if exists != nil {
			if time.Since(*exists) > e.config.TouchRefresh {
				err = e.s3Client.touch(ctx, key)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to touch file")
				}
			}
		} else {
			layerDone := progress.OneOff(ctx, fmt.Sprintf("writing layer %s", l.Blob))
			dt, err := content.ReadBlob(ctx, dgstPair.Provider, dgstPair.Descriptor)
			if err != nil {
				return nil, layerDone(err)
			}
			if err := e.s3Client.saveMutable(ctx, key, dt); err != nil {
				return nil, layerDone(errors.Wrap(err, "error writing layer blob"))
			}
			layerDone(nil)
		}

		la := &v1.LayerAnnotations{
			DiffID:    diffID,
			Size:      dgstPair.Descriptor.Size,
			MediaType: dgstPair.Descriptor.MediaType,
		}
		if v, ok := dgstPair.Descriptor.Annotations["buildkit/createdat"]; ok {
			var t time.Time
			if err := (&t).UnmarshalText([]byte(v)); err != nil {
				return nil, err
			}
			la.CreatedAt = t.UTC()
		}
		cacheConfig.Layers[i].Annotations = la
	}

	dt, err := json.Marshal(cacheConfig)
	if err != nil {
		return nil, err
	}

	for _, name := range e.config.Names {
		if err := e.s3Client.saveMutable(ctx, e.s3Client.manifestKey(name), dt); err != nil {
			return nil, errors.Wrapf(err, "error writing manifest: %s", name)
		}
	}
	return nil, nil
}

// ResolveCacheImporterFunc for s3 cache importer.
func ResolveCacheImporterFunc() remotecache.ResolveCacheImporterFunc {
	return func(ctx context.Context, _ session.Group, attrs map[string]string) (remotecache.Importer, ocispecs.Descriptor, error) {
		config, err := getConfig(attrs)
		if err != nil {
			return nil, ocispecs.Descriptor{}, err
		}
		s3Client, err := newS3Client(ctx, config)
		if err != nil {
			return nil, ocispecs.Descriptor{}, err
		}
		return &importer{s3Client, config}, ocispecs.Descriptor{}, nil
	}
}

type importer struct {
	s3Client *s3Client
	config   Config
}

func (i *importer) makeDescriptorProviderPair(l v1.CacheLayer) (*v1.DescriptorProviderPair, error) {
	if l.Annotations == nil {
		return nil, errors.Errorf("cache layer with missing annotations")
	}
	if l.Annotations.DiffID == "" {
		return nil, errors.Errorf("cache layer with missing diffid")
	}
	annotations := map[string]string{}
	annotations["containerd.io/uncompressed"] = l.Annotations.DiffID.String()
	if !l.Annotations.CreatedAt.IsZero() {
		txt, err := l.Annotations.CreatedAt.MarshalText()
		if err != nil {
			return nil, err
		}
		annotations["buildkit/createdat"] = string(txt)
	}
	return &v1.DescriptorProviderPair{
		Provider: i.s3Client,
		Descriptor: ocispecs.Descriptor{
			MediaType:   l.Annotations.MediaType,
			Digest:      l.Blob,
			Size:        l.Annotations.Size,
			Annotations: annotations,
		},
	}, nil
}

func (i *importer) load(ctx context.Context) (*v1.CacheChains, error) {
	var config v1.CacheConfig
	found, err := i.s3Client.getManifest(ctx, i.s3Client.manifestKey(i.config.Names[0]), &config)
	if err != nil {
		return nil, err
	}
	if !found {
		return v1.NewCacheChains(), nil
	}

	allLayers := v1.DescriptorProvider{}

	for _, l := range config.Layers {
		dpp, err := i.makeDescriptorProviderPair(l)
		if err != nil {
			return nil, err
		}
		allLayers[l.Blob] = *dpp
	}

	cc := v1.NewCacheChains()
	if err := v1.ParseConfig(config, allLayers, cc); err != nil {
		return nil, err
	}
	return cc, nil
}

func (i *importer) Resolve(ctx context.Context, _ ocispecs.Descriptor, id string, w worker.Worker) (solver.CacheManager, error) {
	cc, err := i.load(ctx)
	if err != nil {
		return nil, err
	}

	keysStorage, resultStorage, err := v1.NewCacheKeyStorage(cc, w)
	if err != nil {
		return nil, err
	}

	return solver.NewCacheManager(ctx, id, keysStorage, resultStorage), nil
}

type readerAt struct {
	ReaderAtCloser
	size int64
}

func (r *readerAt) Size() int64 {
	return r.size
}

type s3Client struct {
	*s3.Client
	*manager.Uploader
	bucket          string
	prefix          string
	blobsPrefix     string
	manifestsPrefix string
}

func newS3Client(ctx context.Context, config Config) (*s3Client, error) {
	cfg, err := aws_config.LoadDefaultConfig(ctx, aws_config.WithRegion(config.Region))
	if err != nil {
		return nil, errors.Errorf("Unable to load AWS SDK config, %v", err)
	}
	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		if config.AccessKeyID != "" && config.SecretAccessKey != "" {
			options.Credentials = credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.SecretAccessKey, config.SessionToken)
		}
		if config.EndpointURL != "" {
			options.UsePathStyle = config.UsePathStyle
			options.EndpointResolver = s3.EndpointResolverFromURL(config.EndpointURL)
		}
	})

	return &s3Client{
		Client:          client,
		Uploader:        manager.NewUploader(client),
		bucket:          config.Bucket,
		prefix:          config.Prefix,
		blobsPrefix:     config.BlobsPrefix,
		manifestsPrefix: config.ManifestsPrefix,
	}, nil
}

func (s3Client *s3Client) getManifest(ctx context.Context, key string, config *v1.CacheConfig) (bool, error) {
	input := &s3.GetObjectInput{
		Bucket: &s3Client.bucket,
		Key:    &key,
	}

	output, err := s3Client.GetObject(ctx, input)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	defer output.Body.Close()

	decoder := json.NewDecoder(output.Body)
	if err := decoder.Decode(config); err != nil {
		return false, errors.WithStack(err)
	}

	return true, nil
}

func (s3Client *s3Client) getReader(ctx context.Context, key string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: &s3Client.bucket,
		Key:    &key,
	}

	output, err := s3Client.GetObject(ctx, input)
	if err != nil {
		return nil, err
	}
	return output.Body, nil
}

func (s3Client *s3Client) saveMutable(ctx context.Context, key string, value []byte) error {
	input := &s3.PutObjectInput{
		Bucket: &s3Client.bucket,
		Key:    &key,

		Body: bytes.NewReader(value),
	}
	_, err := s3Client.Upload(ctx, input)
	return err
}

func (s3Client *s3Client) exists(ctx context.Context, key string) (*time.Time, error) {
	input := &s3.HeadObjectInput{
		Bucket: &s3Client.bucket,
		Key:    &key,
	}

	head, err := s3Client.HeadObject(ctx, input)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return head.LastModified, nil
}

func (s3Client *s3Client) touch(ctx context.Context, key string) error {
	copySource := fmt.Sprintf("%s/%s", s3Client.bucket, key)
	cp := &s3.CopyObjectInput{
		Bucket:            &s3Client.bucket,
		CopySource:        &copySource,
		Key:               &key,
		Metadata:          map[string]string{"updated-at": time.Now().String()},
		MetadataDirective: "REPLACE",
	}

	_, err := s3Client.CopyObject(ctx, cp)

	return err
}

func (s3Client *s3Client) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	readerAtCloser := toReaderAtCloser(func(offset int64) (io.ReadCloser, error) {
		return s3Client.getReader(ctx, s3Client.blobKey(desc.Digest))
	})
	return &readerAt{ReaderAtCloser: readerAtCloser, size: desc.Size}, nil
}

func (s3Client *s3Client) manifestKey(name string) string {
	return s3Client.prefix + s3Client.manifestsPrefix + name
}

func (s3Client *s3Client) blobKey(dgst digest.Digest) string {
	return s3Client.prefix + s3Client.blobsPrefix + dgst.String()
}

func isNotFound(err error) bool {
	var errapi smithy.APIError
	return errors.As(err, &errapi) && (errapi.ErrorCode() == "NoSuchKey" || errapi.ErrorCode() == "NotFound")
}
