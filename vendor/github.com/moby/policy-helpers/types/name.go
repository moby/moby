package types

import (
	"strings"
)

const (
	githubPrefix                 = "https://github.com/"
	githubBuilderURIExperimental = githubPrefix + "docker/github-builder-experimental/.github/workflows/"
	githubBuilderURI             = githubPrefix + "docker/github-builder/.github/workflows/"

	githubIssuer     = "https://token.actions.githubusercontent.com"
	googleUserIssuer = "https://accounts.google.com"
	githubUserIssuer = githubPrefix + "login/oauth"

	sigstoreIssuer = "CN=sigstore-intermediate,O=sigstore.dev"
)

func (s SignatureInfo) DetectKind() Kind {
	if isDHI(s) {
		return KindDockerHardenedImage
	}
	if isGithubBuilder(s) {
		return KindDockerGithubBuilder
	}
	if isGithubSelfSigned(s) {
		return KindSelfSignedGithubRepo
	}
	if isSelfSigned(s) {
		return KindSelfSigned
	}
	return KindUntrusted
}

func (s SignatureInfo) Name() string {
	switch s.Kind {
	case KindDockerHardenedImage:
		return s.Kind.String() + " (" + s.DockerReference + ")"

	case KindDockerGithubBuilder:
		n := s.Kind.String()
		if strings.HasPrefix(s.Signer.BuildSignerURI, githubBuilderURIExperimental) {
			n += " Experimental"
		}
		n += " (" + strings.TrimPrefix(s.Signer.SourceRepositoryURI, githubPrefix)

		if v, ok := strings.CutPrefix(s.Signer.SourceRepositoryRef, "refs/heads/"); ok {
			n += "@" + v
		} else if v, ok := strings.CutPrefix(s.Signer.SourceRepositoryRef, "refs/tags/"); ok {
			n += "@" + v
		}
		n += ")"
		return n

	case KindSelfSignedGithubRepo:

		return s.Kind.String() + " (" + strings.TrimPrefix(s.Signer.SourceRepositoryURI, githubPrefix) + ")"

	case KindSelfSigned:
		n := s.Kind.String()
		if s.Signer.RunnerEnvironment != "github-hosted" {
			n += " Local"
		}
		switch s.Signer.Issuer {
		case googleUserIssuer:
			n += " (Google: " + s.Signer.SubjectAlternativeName + ")"
		case githubUserIssuer:
			n += " (GitHub: " + s.Signer.SubjectAlternativeName + ")"
		}
		return n

	default:
		return s.Kind.String()
	}
}

func isDHI(s SignatureInfo) bool {
	if !s.IsDHI {
		return false
	}
	if s.DockerReference == "" {
		return false
	}
	return true
}

func isGithubBuilder(s SignatureInfo) bool {
	if s.Signer == nil {
		return false
	}

	if s.Signer.CertificateIssuer != sigstoreIssuer {
		return false
	}

	isGithubBuilder := strings.HasPrefix(s.Signer.BuildSignerURI, githubBuilderURI)
	isExperimental := strings.HasPrefix(s.Signer.BuildSignerURI, githubBuilderURIExperimental)
	if !isGithubBuilder && !isExperimental {
		return false
	}

	if !strings.HasPrefix(s.Signer.SubjectAlternativeName, githubBuilderURI) && !strings.HasPrefix(s.Signer.SubjectAlternativeName, githubBuilderURIExperimental) {
		return false
	}

	if s.Signer.Issuer != githubIssuer {
		return false
	}

	if !strings.HasPrefix(s.Signer.SourceRepositoryURI, githubPrefix) {
		return false
	}

	if s.Signer.RunnerEnvironment != "github-hosted" {
		return false
	}

	if len(s.Timestamps) == 0 {
		return false
	}

	if s.SignatureType != SignatureBundleV03 {
		return false
	}

	return true
}

func isSelfSigned(s SignatureInfo) bool {
	if s.Signer == nil {
		return false
	}

	if s.Signer.CertificateIssuer != sigstoreIssuer {
		return false
	}

	return true
}

func isGithubSelfSigned(s SignatureInfo) bool {
	if s.Signer == nil {
		return false
	}

	if s.Signer.CertificateIssuer != sigstoreIssuer {
		return false
	}

	if s.Signer.Issuer != githubIssuer {
		return false
	}

	if !strings.HasPrefix(s.Signer.SourceRepositoryURI, "https://github.com/") {
		return false
	}

	signerURIPrefix := s.Signer.SourceRepositoryURI + "/.github/workflows/"
	if !strings.HasPrefix(s.Signer.BuildSignerURI, signerURIPrefix) {
		return false
	}

	if !strings.HasPrefix(s.Signer.SubjectAlternativeName, signerURIPrefix) {
		return false
	}

	if s.Signer.RunnerEnvironment != "github-hosted" {
		return false
	}

	return true
}
