// Package v1 is deprecated and moved to github.com/moby/docker-image-spec/specs-go/v1
//
// Deprecated: use github.com/moby/docker-image-spec/specs-go instead.
package v1

import v1 "github.com/moby/docker-image-spec/specs-go/v1"

// DockerOCIImageMediaType is the media-type used for Docker Image spec images.
//
// Deprecated: use [v1.DockerOCIImageMediaType].
const DockerOCIImageMediaType = v1.DockerOCIImageMediaType

// DockerOCIImage is a ocispec.Image extended with Docker specific Config.
//
// Deprecated: use [v1.DockerOCIImage].
type DockerOCIImage = v1.DockerOCIImage

// DockerOCIImageConfig is a ocispec.ImageConfig extended with Docker specific fields.
//
// Deprecated: use [v1.DockerOCIImageConfig]
type DockerOCIImageConfig = v1.DockerOCIImageConfig

// DockerOCIImageConfigExt contains Docker-specific fields in DockerImageConfig.
//
// Deprecated: use [v1.DockerOCIImageConfigExt].
type DockerOCIImageConfigExt = v1.DockerOCIImageConfigExt

// HealthcheckConfig holds configuration settings for the HEALTHCHECK feature.
//
// Deprecated: use [v1.HealthcheckConfig].
type HealthcheckConfig = v1.HealthcheckConfig
