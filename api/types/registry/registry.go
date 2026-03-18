package registry

import (
	"net/netip"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ServiceConfig stores daemon registry services configuration.
type ServiceConfig struct {
	InsecureRegistryCIDRs []netip.Prefix        `json:"InsecureRegistryCIDRs"`
	IndexConfigs          map[string]*IndexInfo `json:"IndexConfigs"`
	Mirrors               []string
}

// IndexInfo contains information about a registry
//
// RepositoryInfo Examples:
//
//	{
//	  "Index" : {
//	    "Name" : "docker.io",
//	    "Mirrors" : ["https://registry-2.docker.io/v1/", "https://registry-3.docker.io/v1/"],
//	    "Secure" : true,
//	    "Official" : true,
//	  },
//	  "RemoteName" : "library/debian",
//	  "LocalName" : "debian",
//	  "CanonicalName" : "docker.io/debian"
//	  "Official" : true,
//	}
//
//	{
//	  "Index" : {
//	    "Name" : "127.0.0.1:5000",
//	    "Mirrors" : [],
//	    "Secure" : false,
//	    "Official" : false,
//	  },
//	  "RemoteName" : "user/repo",
//	  "LocalName" : "127.0.0.1:5000/user/repo",
//	  "CanonicalName" : "127.0.0.1:5000/user/repo",
//	  "Official" : false,
//	}
type IndexInfo struct {
	// Name is the name of the registry, such as "docker.io"
	Name string
	// Mirrors is a list of mirrors, expressed as URIs
	Mirrors []string
	// Secure is set to false if the registry is part of the list of
	// insecure registries. Insecure registries accept HTTP and/or accept
	// HTTPS with certificates from unknown CAs.
	Secure bool
	// Official indicates whether this is an official registry
	Official bool
}

// DistributionInspect describes the result obtained from contacting the
// registry to retrieve image metadata
type DistributionInspect struct {
	// Descriptor contains information about the manifest, including
	// the content addressable digest
	Descriptor ocispec.Descriptor
	// Platforms contains the list of platforms supported by the image,
	// obtained by parsing the manifest
	Platforms []ocispec.Platform
}
