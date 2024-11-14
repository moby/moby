package exptypes

import commonexptypes "github.com/moby/buildkit/exporter/exptypes"

type ImageExporterOptKey string

// Options keys supported by the image exporter output.
var (
	// Name of the image.
	// Value: string
	OptKeyName ImageExporterOptKey = "name"

	// Push after creating image.
	// Value: bool <true|false>
	OptKeyPush ImageExporterOptKey = "push"

	// Push unnamed image.
	// Value: bool <true|false>
	OptKeyPushByDigest ImageExporterOptKey = "push-by-digest"

	// Allow pushing to insecure HTTP registry.
	// Value: bool <true|false>
	OptKeyInsecure ImageExporterOptKey = "registry.insecure"

	// Unpack image after it's created (containerd).
	// Value: bool <true|false>
	OptKeyUnpack ImageExporterOptKey = "unpack"

	// Fallback image name prefix if image name isn't provided.
	// If used, image will be named as <value>@<digest>
	// Value: string
	OptKeyDanglingPrefix ImageExporterOptKey = "dangling-name-prefix"

	// Creates additional image name with format <name>@<digest>
	// Value: bool <true|false>
	OptKeyNameCanonical ImageExporterOptKey = "name-canonical"

	// Store the resulting image along with all of the content it references.
	// Ignored if the worker doesn't have image store (e.g. OCI worker).
	// Value: bool <true|false>
	OptKeyStore ImageExporterOptKey = "store"

	// Use OCI mediatypes instead of Docker in JSON configs.
	// Value: bool <true|false>
	OptKeyOCITypes ImageExporterOptKey = "oci-mediatypes"

	// Use OCI artifact format for the attestation manifest.
	OptKeyOCIArtifact ImageExporterOptKey = "oci-artifact"

	// Force attestation to be attached.
	// Value: bool <true|false>
	OptKeyForceInlineAttestations ImageExporterOptKey = "attestation-inline"

	// Mark layers as non-distributable if they are found to use a
	// non-distributable media type. When this option is not set, the exporter
	// will change the media type of the layer to a distributable one.
	// Value: bool <true|false>
	OptKeyPreferNondistLayers ImageExporterOptKey = "prefer-nondist-layers"

	// Clamp produced timestamps. For more information see the
	// SOURCE_DATE_EPOCH specification.
	// Value: int (number of seconds since Unix epoch)
	OptKeySourceDateEpoch ImageExporterOptKey = ImageExporterOptKey(commonexptypes.OptKeySourceDateEpoch)

	// Compression type for newly created and cached layers.
	// estargz should be used with OptKeyOCITypes set to true.
	// Value: string <uncompressed|gzip|estargz|zstd>
	OptKeyLayerCompression ImageExporterOptKey = "compression"

	// Force compression on all (including existing) layers.
	// Value: bool <true|false>
	OptKeyForceCompression ImageExporterOptKey = "force-compression"

	// Compression level
	// Value: int (0-9) for gzip and estargz
	// Value: int (0-22) for zstd
	OptKeyCompressionLevel ImageExporterOptKey = "compression-level"

	// Rewrite timestamps in layers to match SOURCE_DATE_EPOCH
	// Value: bool <true|false>
	OptKeyRewriteTimestamp ImageExporterOptKey = "rewrite-timestamp"
)
