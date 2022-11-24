package attestation

const (
	MediaTypeDockerSchema2AttestationType = "application/vnd.in-toto+json"

	DockerAnnotationReferenceType        = "vnd.docker.reference.type"
	DockerAnnotationReferenceDigest      = "vnd.docker.reference.digest"
	DockerAnnotationReferenceDescription = "vnd.docker.reference.description"

	DockerAnnotationReferenceTypeDefault = "attestation-manifest"
)
