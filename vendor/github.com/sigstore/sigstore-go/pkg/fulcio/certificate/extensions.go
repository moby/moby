// Copyright 2023 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file is a verbatim copy of https://github.com/sigstore/fulcio/blob/3707d80bb25330bc7ffbd9702fb401cd643e36fa/pkg/certificate/extensions.go ,
// EXCEPT:
// - the parseExtensions func has been renamed ParseExtensions

package certificate

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
)

var (
	// Deprecated: Use OIDIssuerV2
	OIDIssuer = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}
	// Deprecated: Use OIDBuildTrigger
	OIDGitHubWorkflowTrigger = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 2}
	// Deprecated: Use OIDSourceRepositoryDigest
	OIDGitHubWorkflowSHA = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 3}
	// Deprecated: Use OIDBuildConfigURI or OIDBuildConfigDigest
	OIDGitHubWorkflowName = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 4}
	// Deprecated: Use SourceRepositoryURI
	OIDGitHubWorkflowRepository = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 5}
	// Deprecated: Use OIDSourceRepositoryRef
	OIDGitHubWorkflowRef = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 6}

	OIDOtherName = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 7}
	OIDIssuerV2  = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 8}

	// CI extensions
	OIDBuildSignerURI                      = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 9}
	OIDBuildSignerDigest                   = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 10}
	OIDRunnerEnvironment                   = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 11}
	OIDSourceRepositoryURI                 = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 12}
	OIDSourceRepositoryDigest              = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 13}
	OIDSourceRepositoryRef                 = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 14}
	OIDSourceRepositoryIdentifier          = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 15}
	OIDSourceRepositoryOwnerURI            = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 16}
	OIDSourceRepositoryOwnerIdentifier     = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 17}
	OIDBuildConfigURI                      = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 18}
	OIDBuildConfigDigest                   = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 19}
	OIDBuildTrigger                        = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 20}
	OIDRunInvocationURI                    = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 21}
	OIDSourceRepositoryVisibilityAtSigning = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 22}
)

// Extensions contains all custom x509 extensions defined by Fulcio
type Extensions struct {
	// NB: New extensions must be added here and documented
	// at docs/oidc-info.md

	// The OIDC issuer. Should match `iss` claim of ID token or, in the case of
	// a federated login like Dex it should match the issuer URL of the
	// upstream issuer. The issuer is not set the extensions are invalid and
	// will fail to render.
	Issuer string `json:"issuer,omitempty"` // OID 1.3.6.1.4.1.57264.1.8 and 1.3.6.1.4.1.57264.1.1 (Deprecated)

	// Deprecated
	// Triggering event of the Github Workflow. Matches the `event_name` claim of ID
	// tokens from Github Actions
	GithubWorkflowTrigger string `json:"githubWorkflowTrigger,omitempty"` // OID 1.3.6.1.4.1.57264.1.2

	// Deprecated
	// SHA of git commit being built in Github Actions. Matches the `sha` claim of ID
	// tokens from Github Actions
	GithubWorkflowSHA string `json:"githubWorkflowSHA,omitempty"` //nolint:tagliatelle // OID 1.3.6.1.4.1.57264.1.3

	// Deprecated
	// Name of Github Actions Workflow. Matches the `workflow` claim of the ID
	// tokens from Github Actions
	GithubWorkflowName string `json:"githubWorkflowName,omitempty"` // OID 1.3.6.1.4.1.57264.1.4

	// Deprecated
	// Repository of the Github Actions Workflow. Matches the `repository` claim of the ID
	// tokens from Github Actions
	GithubWorkflowRepository string `json:"githubWorkflowRepository,omitempty"` // OID 1.3.6.1.4.1.57264.1.5

	// Deprecated
	// Git Ref of the Github Actions Workflow. Matches the `ref` claim of the ID tokens
	// from Github Actions
	GithubWorkflowRef string `json:"githubWorkflowRef,omitempty"` // 1.3.6.1.4.1.57264.1.6

	// Reference to specific build instructions that are responsible for signing.
	BuildSignerURI string `json:"buildSignerURI,omitempty"` //nolint:tagliatelle // 1.3.6.1.4.1.57264.1.9

	// Immutable reference to the specific version of the build instructions that is responsible for signing.
	BuildSignerDigest string `json:"buildSignerDigest,omitempty"` // 1.3.6.1.4.1.57264.1.10

	// Specifies whether the build took place in platform-hosted cloud infrastructure or customer/self-hosted infrastructure.
	RunnerEnvironment string `json:"runnerEnvironment,omitempty"` // 1.3.6.1.4.1.57264.1.11

	// Source repository URL that the build was based on.
	SourceRepositoryURI string `json:"sourceRepositoryURI,omitempty"` //nolint:tagliatelle  // 1.3.6.1.4.1.57264.1.12

	// Immutable reference to a specific version of the source code that the build was based upon.
	SourceRepositoryDigest string `json:"sourceRepositoryDigest,omitempty"` // 1.3.6.1.4.1.57264.1.13

	// Source Repository Ref that the build run was based upon.
	SourceRepositoryRef string `json:"sourceRepositoryRef,omitempty"` // 1.3.6.1.4.1.57264.1.14

	// Immutable identifier for the source repository the workflow was based upon.
	SourceRepositoryIdentifier string `json:"sourceRepositoryIdentifier,omitempty"` // 1.3.6.1.4.1.57264.1.15

	// Source repository owner URL of the owner of the source repository that the build was based on.
	SourceRepositoryOwnerURI string `json:"sourceRepositoryOwnerURI,omitempty"` //nolint:tagliatelle // 1.3.6.1.4.1.57264.1.16

	// Immutable identifier for the owner of the source repository that the workflow was based upon.
	SourceRepositoryOwnerIdentifier string `json:"sourceRepositoryOwnerIdentifier,omitempty"` // 1.3.6.1.4.1.57264.1.17

	// Build Config URL to the top-level/initiating build instructions.
	BuildConfigURI string `json:"buildConfigURI,omitempty"` //nolint:tagliatelle // 1.3.6.1.4.1.57264.1.18

	// Immutable reference to the specific version of the top-level/initiating build instructions.
	BuildConfigDigest string `json:"buildConfigDigest,omitempty"` // 1.3.6.1.4.1.57264.1.19

	// Event or action that initiated the build.
	BuildTrigger string `json:"buildTrigger,omitempty"` // 1.3.6.1.4.1.57264.1.20

	// Run Invocation URL to uniquely identify the build execution.
	RunInvocationURI string `json:"runInvocationURI,omitempty"` //nolint:tagliatelle // 1.3.6.1.4.1.57264.1.21

	// Source repository visibility at the time of signing the certificate.
	SourceRepositoryVisibilityAtSigning string `json:"sourceRepositoryVisibilityAtSigning,omitempty"` // 1.3.6.1.4.1.57264.1.22
}

func ParseExtensions(ext []pkix.Extension) (Extensions, error) {
	out := Extensions{}

	for _, e := range ext {
		switch {
		// BEGIN: Deprecated
		case e.Id.Equal(OIDIssuer):
			out.Issuer = string(e.Value)
		case e.Id.Equal(OIDGitHubWorkflowTrigger):
			out.GithubWorkflowTrigger = string(e.Value)
		case e.Id.Equal(OIDGitHubWorkflowSHA):
			out.GithubWorkflowSHA = string(e.Value)
		case e.Id.Equal(OIDGitHubWorkflowName):
			out.GithubWorkflowName = string(e.Value)
		case e.Id.Equal(OIDGitHubWorkflowRepository):
			out.GithubWorkflowRepository = string(e.Value)
		case e.Id.Equal(OIDGitHubWorkflowRef):
			out.GithubWorkflowRef = string(e.Value)
		// END: Deprecated
		case e.Id.Equal(OIDIssuerV2):
			if err := ParseDERString(e.Value, &out.Issuer); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDBuildSignerURI):
			if err := ParseDERString(e.Value, &out.BuildSignerURI); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDBuildSignerDigest):
			if err := ParseDERString(e.Value, &out.BuildSignerDigest); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDRunnerEnvironment):
			if err := ParseDERString(e.Value, &out.RunnerEnvironment); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDSourceRepositoryURI):
			if err := ParseDERString(e.Value, &out.SourceRepositoryURI); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDSourceRepositoryDigest):
			if err := ParseDERString(e.Value, &out.SourceRepositoryDigest); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDSourceRepositoryRef):
			if err := ParseDERString(e.Value, &out.SourceRepositoryRef); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDSourceRepositoryIdentifier):
			if err := ParseDERString(e.Value, &out.SourceRepositoryIdentifier); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDSourceRepositoryOwnerURI):
			if err := ParseDERString(e.Value, &out.SourceRepositoryOwnerURI); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDSourceRepositoryOwnerIdentifier):
			if err := ParseDERString(e.Value, &out.SourceRepositoryOwnerIdentifier); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDBuildConfigURI):
			if err := ParseDERString(e.Value, &out.BuildConfigURI); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDBuildConfigDigest):
			if err := ParseDERString(e.Value, &out.BuildConfigDigest); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDBuildTrigger):
			if err := ParseDERString(e.Value, &out.BuildTrigger); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDRunInvocationURI):
			if err := ParseDERString(e.Value, &out.RunInvocationURI); err != nil {
				return Extensions{}, err
			}
		case e.Id.Equal(OIDSourceRepositoryVisibilityAtSigning):
			if err := ParseDERString(e.Value, &out.SourceRepositoryVisibilityAtSigning); err != nil {
				return Extensions{}, err
			}
		}
	}

	// We only ever return nil, but leaving error in place so that we can add
	// more complex parsing of fields in a backwards compatible way if needed.
	return out, nil
}

// ParseDERString decodes a DER-encoded string and puts the value in parsedVal.
// Returns an error if the unmarshalling fails or if there are trailing bytes in the encoding.
func ParseDERString(val []byte, parsedVal *string) error {
	rest, err := asn1.Unmarshal(val, parsedVal)
	if err != nil {
		return fmt.Errorf("unexpected error unmarshalling DER-encoded string: %w", err)
	}
	if len(rest) != 0 {
		return errors.New("unexpected trailing bytes in DER-encoded string")
	}
	return nil
}
