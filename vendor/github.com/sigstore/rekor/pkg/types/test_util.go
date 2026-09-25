/*
Copyright © 2021 The Sigstore Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"context"

	"github.com/go-openapi/strfmt"

	"github.com/sigstore/rekor/pkg/generated/models"
	pkitypes "github.com/sigstore/rekor/pkg/pki/pkitypes"
)

type BaseUnmarshalTester struct{}

func (u BaseUnmarshalTester) NewEntry() EntryImpl {
	return &BaseUnmarshalTester{}
}

func (u BaseUnmarshalTester) ArtifactHash() (string, error) {
	return "", nil
}

func (u BaseUnmarshalTester) Verifiers() ([]pkitypes.PublicKey, error) {
	return nil, nil
}

func (u BaseUnmarshalTester) APIVersion() string {
	return "2.0.1"
}

func (u BaseUnmarshalTester) IndexKeys() ([]string, error) {
	return []string{}, nil
}

func (u BaseUnmarshalTester) Canonicalize(_ context.Context) ([]byte, error) {
	return nil, nil
}

func (u BaseUnmarshalTester) Unmarshal(_ models.ProposedEntry) error {
	return nil
}

func (u BaseUnmarshalTester) Validate() error {
	return nil
}

func (u BaseUnmarshalTester) AttestationKey() string {
	return ""
}

func (u BaseUnmarshalTester) AttestationKeyValue() (string, []byte) {
	return "", nil
}

func (u BaseUnmarshalTester) CreateFromArtifactProperties(_ context.Context, _ ArtifactProperties) (models.ProposedEntry, error) {
	return nil, nil
}

func (u BaseUnmarshalTester) Insertable() (bool, error) {
	return false, nil
}

type BaseProposedEntryTester struct{}

func (b BaseProposedEntryTester) Kind() string {
	return "nil"
}

func (b BaseProposedEntryTester) SetKind(_ string) {

}

func (b BaseProposedEntryTester) Validate(_ strfmt.Registry) error {
	return nil
}

func (b BaseProposedEntryTester) ContextValidate(_ context.Context, _ strfmt.Registry) error {
	return nil
}
