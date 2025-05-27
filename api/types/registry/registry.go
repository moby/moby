// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package registry // import "github.com/docker/docker/api/types/registry"

import (
	"encoding/json"
	"net"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ServiceConfig stores daemon registry services configuration.
type ServiceConfig struct {
	AllowNondistributableArtifactsCIDRs     []*NetIPNet `json:"AllowNondistributableArtifactsCIDRs,omitempty"`     // Deprecated: non-distributable artifacts are deprecated and enabled by default. This field will be removed in the next release.
	AllowNondistributableArtifactsHostnames []string    `json:"AllowNondistributableArtifactsHostnames,omitempty"` // Deprecated: non-distributable artifacts are deprecated and enabled by default. This field will be removed in the next release.

	InsecureRegistryCIDRs []*NetIPNet           `json:"InsecureRegistryCIDRs"`
	IndexConfigs          map[string]*IndexInfo `json:"IndexConfigs"`
	Mirrors               []string

	// ExtraFields is for internal use to include deprecated fields on older API versions.
	ExtraFields map[string]any `json:"-"`
}

// MarshalJSON implements a custom marshaler to include legacy fields
// in API responses.
func (sc *ServiceConfig) MarshalJSON() ([]byte, error) {
	type tmp ServiceConfig
	base, err := json.Marshal((*tmp)(sc))
	if err != nil {
		return nil, err
	}
	var merged map[string]any
	_ = json.Unmarshal(base, &merged)

	for k, v := range sc.ExtraFields {
		merged[k] = v
	}
	return json.Marshal(merged)
}

// NetIPNet is the net.IPNet type, which can be marshalled and
// unmarshalled to JSON
type NetIPNet net.IPNet

// String returns the CIDR notation of ipnet
func (ipnet *NetIPNet) String() string {
	return (*net.IPNet)(ipnet).String()
}

// MarshalJSON returns the JSON representation of the IPNet
func (ipnet *NetIPNet) MarshalJSON() ([]byte, error) {
	return json.Marshal((*net.IPNet)(ipnet).String())
}

// UnmarshalJSON sets the IPNet from a byte array of JSON
func (ipnet *NetIPNet) UnmarshalJSON(b []byte) error {
	var ipnetStr string
	if err := json.Unmarshal(b, &ipnetStr); err != nil {
		return err
	}
	_, cidr, err := net.ParseCIDR(ipnetStr)
	if err != nil {
		return err
	}
	*ipnet = NetIPNet(*cidr)
	return nil
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
