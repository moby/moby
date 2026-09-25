//
// Copyright 2021 The Sigstore Authors.
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

package hashedrekord

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag/conv"

	"github.com/sigstore/rekor/pkg/generated/models"
	"github.com/sigstore/rekor/pkg/internal/log"
	pkitypes "github.com/sigstore/rekor/pkg/pki/pkitypes"
	"github.com/sigstore/rekor/pkg/pki/x509"
	"github.com/sigstore/rekor/pkg/types"
	hashedrekord "github.com/sigstore/rekor/pkg/types/hashedrekord"
	"github.com/sigstore/rekor/pkg/util"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

const (
	APIVERSION = "0.0.1"
)

func init() {
	if err := hashedrekord.VersionMap.SetEntryFactory(APIVERSION, NewEntry); err != nil {
		log.Logger.Panic(err)
	}
}

type V001Entry struct {
	HashedRekordObj models.HashedrekordV001Schema
}

func (v V001Entry) APIVersion() string {
	return APIVERSION
}

func NewEntry() types.EntryImpl {
	return &V001Entry{}
}

func (v V001Entry) IndexKeys() ([]string, error) {
	var result []string

	key := v.HashedRekordObj.Signature.PublicKey.Content
	keyHash := sha256.Sum256(key)
	result = append(result, strings.ToLower(hex.EncodeToString(keyHash[:])))

	pub, err := x509.NewPublicKey(bytes.NewReader(key))
	if err != nil {
		return nil, err
	}
	result = append(result, pub.Subjects()...)

	if v.HashedRekordObj.Data.Hash != nil {
		hashKey := strings.ToLower(fmt.Sprintf("%s:%s", *v.HashedRekordObj.Data.Hash.Algorithm, *v.HashedRekordObj.Data.Hash.Value))
		result = append(result, hashKey)
	}

	return result, nil
}

// DecodeEntry performs direct decode into the provided output pointer
// without mutating the receiver on error.
func DecodeEntry(input any, output *models.HashedrekordV001Schema) error {
	if output == nil {
		return fmt.Errorf("nil output *models.HashedrekordV001Schema")
	}
	var m models.HashedrekordV001Schema
	// Single type-switch with map fast path
	switch data := input.(type) {
	case map[string]any:
		mm := data
		if sigRaw, ok := mm["signature"].(map[string]any); ok {
			m.Signature = &models.HashedrekordV001SchemaSignature{}
			if c, ok := sigRaw["content"].(string); ok && c != "" {
				outb := make([]byte, base64.StdEncoding.DecodedLen(len(c)))
				n, err := base64.StdEncoding.Decode(outb, []byte(c))
				if err != nil {
					return fmt.Errorf("failed parsing base64 data for signature content: %w", err)
				}
				m.Signature.Content = outb[:n]
			}
			if pkRaw, ok := sigRaw["publicKey"].(map[string]any); ok {
				m.Signature.PublicKey = &models.HashedrekordV001SchemaSignaturePublicKey{}
				if c, ok := pkRaw["content"].(string); ok && c != "" {
					outb := make([]byte, base64.StdEncoding.DecodedLen(len(c)))
					n, err := base64.StdEncoding.Decode(outb, []byte(c))
					if err != nil {
						return fmt.Errorf("failed parsing base64 data for public key content: %w", err)
					}
					m.Signature.PublicKey.Content = outb[:n]
				}
			}
		}
		if dataRaw, ok := mm["data"].(map[string]any); ok {
			if hRaw, ok := dataRaw["hash"].(map[string]any); ok {
				m.Data = &models.HashedrekordV001SchemaData{Hash: &models.HashedrekordV001SchemaDataHash{}}
				if alg, ok := hRaw["algorithm"].(string); ok {
					m.Data.Hash.Algorithm = &alg
				}
				if val, ok := hRaw["value"].(string); ok {
					m.Data.Hash.Value = &val
				}
			}
		}
		*output = m
		return nil
	case *models.HashedrekordV001Schema:
		if data == nil {
			return fmt.Errorf("nil *models.HashedrekordV001Schema")
		}
		*output = *data
		return nil
	case models.HashedrekordV001Schema:
		*output = data
		return nil
	default:
		return fmt.Errorf("unsupported input type %T for DecodeEntry", input)
	}
}

func (v *V001Entry) Unmarshal(pe models.ProposedEntry) error {
	rekord, ok := pe.(*models.Hashedrekord)
	if !ok {
		return errors.New("cannot unmarshal non Rekord v0.0.1 type")
	}

	if err := DecodeEntry(rekord.Spec, &v.HashedRekordObj); err != nil {
		return err
	}

	// field validation
	if err := v.HashedRekordObj.Validate(strfmt.Default); err != nil {
		return err
	}

	// cross field validation
	_, _, err := v.validate()
	return err
}

func (v *V001Entry) Canonicalize(_ context.Context) ([]byte, error) {
	sigObj, keyObj, err := v.validate()
	if err != nil {
		return nil, &types.InputValidationError{Err: err}
	}

	canonicalEntry := models.HashedrekordV001Schema{}

	// need to canonicalize signature & key content
	canonicalEntry.Signature = &models.HashedrekordV001SchemaSignature{}
	canonicalEntry.Signature.Content, err = sigObj.CanonicalValue()
	if err != nil {
		return nil, err
	}

	// key URL (if known) is not set deliberately
	canonicalEntry.Signature.PublicKey = &models.HashedrekordV001SchemaSignaturePublicKey{}
	canonicalEntry.Signature.PublicKey.Content, err = keyObj.CanonicalValue()
	if err != nil {
		return nil, err
	}

	canonicalEntry.Data = &models.HashedrekordV001SchemaData{}
	canonicalEntry.Data.Hash = v.HashedRekordObj.Data.Hash
	// data content is not set deliberately

	v.HashedRekordObj = canonicalEntry
	// wrap in valid object with kind and apiVersion set
	rekordObj := models.Hashedrekord{}
	rekordObj.APIVersion = conv.Pointer(APIVERSION)
	rekordObj.Spec = &canonicalEntry

	return json.Marshal(&rekordObj)
}

// validate performs cross-field validation for fields in object
func (v *V001Entry) validate() (pkitypes.Signature, pkitypes.PublicKey, error) {
	sig := v.HashedRekordObj.Signature
	if sig == nil {
		return nil, nil, &types.InputValidationError{Err: errors.New("missing signature")}
	}
	// Hashed rekord type only works for x509 signature types
	sigObj, err := x509.NewSignatureWithOpts(bytes.NewReader(sig.Content), options.WithED25519ph())
	if err != nil {
		return nil, nil, &types.InputValidationError{Err: err}
	}

	key := sig.PublicKey
	if key == nil {
		return nil, nil, &types.InputValidationError{Err: errors.New("missing public key")}
	}
	keyObj, err := x509.NewPublicKey(bytes.NewReader(key.Content))
	if err != nil {
		return nil, nil, &types.InputValidationError{Err: err}
	}

	data := v.HashedRekordObj.Data
	if data == nil {
		return nil, nil, &types.InputValidationError{Err: errors.New("missing data")}
	}

	hash := data.Hash
	if hash == nil {
		return nil, nil, &types.InputValidationError{Err: errors.New("missing hash")}
	}

	var alg crypto.Hash
	switch conv.Value(hash.Algorithm) {
	case models.HashedrekordV001SchemaDataHashAlgorithmSha384:
		if len(*hash.Value) != crypto.SHA384.Size()*2 {
			return nil, nil, &types.InputValidationError{Err: errors.New("invalid value for hash")}
		}
		alg = crypto.SHA384
	case models.HashedrekordV001SchemaDataHashAlgorithmSha512:
		if len(*hash.Value) != crypto.SHA512.Size()*2 {
			return nil, nil, &types.InputValidationError{Err: errors.New("invalid value for hash")}
		}
		alg = crypto.SHA512
	default:
		if len(*hash.Value) != crypto.SHA256.Size()*2 {
			return nil, nil, &types.InputValidationError{Err: errors.New("invalid value for hash")}
		}
		alg = crypto.SHA256
	}

	decoded, err := hex.DecodeString(*hash.Value)
	if err != nil {
		return nil, nil, err
	}
	if err := sigObj.Verify(nil, keyObj, options.WithDigest(decoded), options.WithCryptoSignerOpts(alg)); err != nil {
		return nil, nil, &types.InputValidationError{Err: fmt.Errorf("verifying signature: %w", err)}
	}

	return sigObj, keyObj, nil
}

func getDataHashAlgorithm(hashAlgorithm crypto.Hash) string {
	switch hashAlgorithm {
	case crypto.SHA384:
		return models.HashedrekordV001SchemaDataHashAlgorithmSha384
	case crypto.SHA512:
		return models.HashedrekordV001SchemaDataHashAlgorithmSha512
	default:
		return models.HashedrekordV001SchemaDataHashAlgorithmSha256
	}
}

func (v V001Entry) CreateFromArtifactProperties(_ context.Context, props types.ArtifactProperties) (models.ProposedEntry, error) {
	returnVal := models.Hashedrekord{}
	re := V001Entry{}

	// we will need artifact, public-key, signature
	re.HashedRekordObj.Data = &models.HashedrekordV001SchemaData{}

	var err error

	if props.PKIFormat != "x509" {
		return nil, errors.New("hashedrekord entries can only be created for artifacts signed with x509-based PKI")
	}

	re.HashedRekordObj.Signature = &models.HashedrekordV001SchemaSignature{}
	sigBytes := props.SignatureBytes
	if sigBytes == nil {
		if props.SignaturePath == nil {
			return nil, errors.New("a detached signature must be provided")
		}
		sigBytes, err = os.ReadFile(filepath.Clean(props.SignaturePath.Path))
		if err != nil {
			return nil, fmt.Errorf("error reading signature file: %w", err)
		}
	}
	re.HashedRekordObj.Signature.Content = strfmt.Base64(sigBytes)

	re.HashedRekordObj.Signature.PublicKey = &models.HashedrekordV001SchemaSignaturePublicKey{}
	publicKeyBytes := props.PublicKeyBytes
	if len(publicKeyBytes) == 0 {
		if len(props.PublicKeyPaths) != 1 {
			return nil, errors.New("only one public key must be provided to verify detached signature")
		}
		keyBytes, err := os.ReadFile(filepath.Clean(props.PublicKeyPaths[0].Path))
		if err != nil {
			return nil, fmt.Errorf("error reading public key file: %w", err)
		}
		publicKeyBytes = append(publicKeyBytes, keyBytes)
	} else if len(publicKeyBytes) != 1 {
		return nil, errors.New("only one public key must be provided")
	}

	hashAlgorithm, hashValue := util.UnprefixSHA(props.ArtifactHash)
	re.HashedRekordObj.Signature.PublicKey.Content = strfmt.Base64(publicKeyBytes[0])
	re.HashedRekordObj.Data.Hash = &models.HashedrekordV001SchemaDataHash{
		Algorithm: conv.Pointer(getDataHashAlgorithm(hashAlgorithm)),
		Value:     conv.Pointer(hashValue),
	}

	if _, _, err := re.validate(); err != nil {
		return nil, err
	}

	returnVal.APIVersion = conv.Pointer(re.APIVersion())
	returnVal.Spec = re.HashedRekordObj

	return &returnVal, nil
}

func (v V001Entry) Verifiers() ([]pkitypes.PublicKey, error) {
	if v.HashedRekordObj.Signature == nil || v.HashedRekordObj.Signature.PublicKey == nil || v.HashedRekordObj.Signature.PublicKey.Content == nil {
		return nil, errors.New("hashedrekord v0.0.1 entry not initialized")
	}
	key, err := x509.NewPublicKey(bytes.NewReader(v.HashedRekordObj.Signature.PublicKey.Content))
	if err != nil {
		return nil, err
	}
	return []pkitypes.PublicKey{key}, nil
}

func (v V001Entry) ArtifactHash() (string, error) {
	if v.HashedRekordObj.Data == nil || v.HashedRekordObj.Data.Hash == nil || v.HashedRekordObj.Data.Hash.Value == nil || v.HashedRekordObj.Data.Hash.Algorithm == nil {
		return "", errors.New("hashedrekord v0.0.1 entry not initialized")
	}
	return strings.ToLower(fmt.Sprintf("%s:%s", *v.HashedRekordObj.Data.Hash.Algorithm, *v.HashedRekordObj.Data.Hash.Value)), nil
}

func (v V001Entry) Insertable() (bool, error) {
	if v.HashedRekordObj.Signature == nil {
		return false, errors.New("missing signature property")
	}
	if len(v.HashedRekordObj.Signature.Content) == 0 {
		return false, errors.New("missing signature content")
	}
	if v.HashedRekordObj.Signature.PublicKey == nil {
		return false, errors.New("missing publicKey property")
	}
	if len(v.HashedRekordObj.Signature.PublicKey.Content) == 0 {
		return false, errors.New("missing publicKey content")
	}
	if v.HashedRekordObj.Data == nil {
		return false, errors.New("missing data property")
	}
	if v.HashedRekordObj.Data.Hash == nil {
		return false, errors.New("missing hash property")
	}
	if v.HashedRekordObj.Data.Hash.Algorithm == nil {
		return false, errors.New("missing hash algorithm")
	}
	if v.HashedRekordObj.Data.Hash.Value == nil {
		return false, errors.New("missing hash value")
	}
	return true, nil
}
