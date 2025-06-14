package in_toto

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/secure-systems-lab/go-securesystemslib/cjson"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

/*
KeyVal contains the actual values of a key, as opposed to key metadata such as
a key identifier or key type.  For RSA keys, the key value is a pair of public
and private keys in PEM format stored as strings.  For public keys the Private
field may be an empty string.
*/
type KeyVal struct {
	Private     string `json:"private,omitempty"`
	Public      string `json:"public"`
	Certificate string `json:"certificate,omitempty"`
}

/*
Key represents a generic in-toto key that contains key metadata, such as an
identifier, supported hash algorithms to create the identifier, the key type
and the supported signature scheme, and the actual key value.
*/
type Key struct {
	KeyID               string   `json:"keyid"`
	KeyIDHashAlgorithms []string `json:"keyid_hash_algorithms"`
	KeyType             string   `json:"keytype"`
	KeyVal              KeyVal   `json:"keyval"`
	Scheme              string   `json:"scheme"`
}

// ErrEmptyKeyField will be thrown if a field in our Key struct is empty.
var ErrEmptyKeyField = errors.New("empty field in key")

// ErrInvalidHexString will be thrown, if a string doesn't match a hex string.
var ErrInvalidHexString = errors.New("invalid hex string")

// ErrSchemeKeyTypeMismatch will be thrown, if the given scheme and key type are not supported together.
var ErrSchemeKeyTypeMismatch = errors.New("the scheme and key type are not supported together")

// ErrUnsupportedKeyIDHashAlgorithms will be thrown, if the specified KeyIDHashAlgorithms is not supported.
var ErrUnsupportedKeyIDHashAlgorithms = errors.New("the given keyID hash algorithm is not supported")

// ErrKeyKeyTypeMismatch will be thrown, if the specified keyType does not match the key
var ErrKeyKeyTypeMismatch = errors.New("the given key does not match its key type")

// ErrNoPublicKey gets returned when the private key value is not empty.
var ErrNoPublicKey = errors.New("the given key is not a public key")

// ErrCurveSizeSchemeMismatch gets returned, when the scheme and curve size are incompatible
// for example: curve size = "521" and scheme = "ecdsa-sha2-nistp224"
var ErrCurveSizeSchemeMismatch = errors.New("the scheme does not match the curve size")

/*
matchEcdsaScheme checks if the scheme suffix, matches the ecdsa key
curve size. We do not need a full regex match here, because
our validateKey functions are already checking for a valid scheme string.
*/
func matchEcdsaScheme(curveSize int, scheme string) error {
	if !strings.HasSuffix(scheme, strconv.Itoa(curveSize)) {
		return ErrCurveSizeSchemeMismatch
	}
	return nil
}

/*
validateHexString is used to validate that a string passed to it contains
only valid hexadecimal characters.
*/
func validateHexString(str string) error {
	formatCheck, _ := regexp.MatchString("^[a-fA-F0-9]+$", str)
	if !formatCheck {
		return fmt.Errorf("%w: %s", ErrInvalidHexString, str)
	}
	return nil
}

/*
validateKeyVal validates the KeyVal struct. In case of an ed25519 key,
it will check for a hex string for private and public key. In any other
case, validateKeyVal will try to decode the PEM block. If this succeeds,
we have a valid PEM block in our KeyVal struct. On success it will return nil
on failure it will return the corresponding error. This can be either
an ErrInvalidHexString, an ErrNoPEMBlock or an ErrUnsupportedKeyType
if the KeyType is unknown.
*/
func validateKeyVal(key Key) error {
	switch key.KeyType {
	case ed25519KeyType:
		// We cannot use matchPublicKeyKeyType or matchPrivateKeyKeyType here,
		// because we retrieve the key not from PEM. Hence we are dealing with
		// plain ed25519 key bytes. These bytes can't be typechecked like in the
		// matchKeyKeytype functions.
		err := validateHexString(key.KeyVal.Public)
		if err != nil {
			return err
		}
		if key.KeyVal.Private != "" {
			err := validateHexString(key.KeyVal.Private)
			if err != nil {
				return err
			}
		}
	case rsaKeyType, ecdsaKeyType:
		// We do not need the pemData here, so we can throw it away via '_'
		_, parsedKey, err := decodeAndParse([]byte(key.KeyVal.Public))
		if err != nil {
			return err
		}
		err = matchPublicKeyKeyType(parsedKey, key.KeyType)
		if err != nil {
			return err
		}
		if key.KeyVal.Private != "" {
			// We do not need the pemData here, so we can throw it away via '_'
			_, parsedKey, err := decodeAndParse([]byte(key.KeyVal.Private))
			if err != nil {
				return err
			}
			err = matchPrivateKeyKeyType(parsedKey, key.KeyType)
			if err != nil {
				return err
			}
		}
	default:
		return ErrUnsupportedKeyType
	}
	return nil
}

/*
matchPublicKeyKeyType validates an interface if it can be asserted to a
the RSA or ECDSA public key type. We can only check RSA and ECDSA this way,
because we are storing them in PEM format. Ed25519 keys are stored as plain
ed25519 keys encoded as hex strings, thus we have no metadata for them.
This function will return nil on success. If the key type does not match
it will return an ErrKeyKeyTypeMismatch.
*/
func matchPublicKeyKeyType(key interface{}, keyType string) error {
	switch key.(type) {
	case *rsa.PublicKey:
		if keyType != rsaKeyType {
			return ErrKeyKeyTypeMismatch
		}
	case *ecdsa.PublicKey:
		if keyType != ecdsaKeyType {
			return ErrKeyKeyTypeMismatch
		}
	default:
		return ErrInvalidKey
	}
	return nil
}

/*
matchPrivateKeyKeyType validates an interface if it can be asserted to a
the RSA or ECDSA private key type. We can only check RSA and ECDSA this way,
because we are storing them in PEM format. Ed25519 keys are stored as plain
ed25519 keys encoded as hex strings, thus we have no metadata for them.
This function will return nil on success. If the key type does not match
it will return an ErrKeyKeyTypeMismatch.
*/
func matchPrivateKeyKeyType(key interface{}, keyType string) error {
	// we can only check RSA and ECDSA this way, because we are storing them in PEM
	// format. ed25519 keys are stored as plain ed25519 keys encoded as hex strings
	// so we have no metadata for them.
	switch key.(type) {
	case *rsa.PrivateKey:
		if keyType != rsaKeyType {
			return ErrKeyKeyTypeMismatch
		}
	case *ecdsa.PrivateKey:
		if keyType != ecdsaKeyType {
			return ErrKeyKeyTypeMismatch
		}
	default:
		return ErrInvalidKey
	}
	return nil
}

/*
matchKeyTypeScheme checks if the specified scheme matches our specified
keyType. If the keyType is not supported it will return an
ErrUnsupportedKeyType. If the keyType and scheme do not match it will return
an ErrSchemeKeyTypeMismatch. If the specified keyType and scheme are
compatible matchKeyTypeScheme will return nil.
*/
func matchKeyTypeScheme(key Key) error {
	switch key.KeyType {
	case rsaKeyType:
		for _, scheme := range getSupportedRSASchemes() {
			if key.Scheme == scheme {
				return nil
			}
		}
	case ed25519KeyType:
		for _, scheme := range getSupportedEd25519Schemes() {
			if key.Scheme == scheme {
				return nil
			}
		}
	case ecdsaKeyType:
		for _, scheme := range getSupportedEcdsaSchemes() {
			if key.Scheme == scheme {
				return nil
			}
		}
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedKeyType, key.KeyType)
	}
	return ErrSchemeKeyTypeMismatch
}

/*
validateKey checks the outer key object (everything, except the KeyVal struct).
It verifies the keyID for being a hex string and checks for empty fields.
On success it will return nil, on error it will return the corresponding error.
Either: ErrEmptyKeyField or ErrInvalidHexString.
*/
func validateKey(key Key) error {
	err := validateHexString(key.KeyID)
	if err != nil {
		return err
	}
	// This probably can be done more elegant with reflection
	// but we care about performance, do we?!
	if key.KeyType == "" {
		return fmt.Errorf("%w: keytype", ErrEmptyKeyField)
	}
	if key.KeyVal.Public == "" && key.KeyVal.Certificate == "" {
		return fmt.Errorf("%w: keyval.public and keyval.certificate cannot both be blank", ErrEmptyKeyField)
	}
	if key.Scheme == "" {
		return fmt.Errorf("%w: scheme", ErrEmptyKeyField)
	}
	err = matchKeyTypeScheme(key)
	if err != nil {
		return err
	}
	// only check for supported KeyIDHashAlgorithms, if the variable has been set
	if key.KeyIDHashAlgorithms != nil {
		supportedKeyIDHashAlgorithms := getSupportedKeyIDHashAlgorithms()
		if !supportedKeyIDHashAlgorithms.IsSubSet(NewSet(key.KeyIDHashAlgorithms...)) {
			return fmt.Errorf("%w: %#v, supported are: %#v", ErrUnsupportedKeyIDHashAlgorithms, key.KeyIDHashAlgorithms, getSupportedKeyIDHashAlgorithms())
		}
	}
	return nil
}

/*
validatePublicKey is a wrapper around validateKey. It test if the private key
value in the key is empty and then validates the key via calling validateKey.
On success it will return nil, on error it will return an ErrNoPublicKey error.
*/
func validatePublicKey(key Key) error {
	if key.KeyVal.Private != "" {
		return ErrNoPublicKey
	}
	err := validateKey(key)
	if err != nil {
		return err
	}
	return nil
}

/*
Signature represents a generic in-toto signature that contains the identifier
of the Key, which was used to create the signature and the signature data.  The
used signature scheme is found in the corresponding Key.
*/
type Signature struct {
	KeyID       string `json:"keyid"`
	Sig         string `json:"sig"`
	Certificate string `json:"cert,omitempty"`
}

// GetCertificate returns the parsed x509 certificate attached to the signature,
// if it exists.
func (sig Signature) GetCertificate() (Key, error) {
	key := Key{}
	if len(sig.Certificate) == 0 {
		return key, errors.New("Signature has empty Certificate")
	}

	err := key.LoadKeyReaderDefaults(strings.NewReader(sig.Certificate))
	return key, err
}

/*
validateSignature is a function used to check if a passed signature is valid,
by inspecting the key ID and the signature itself.
*/
func validateSignature(signature Signature) error {
	if err := validateHexString(signature.KeyID); err != nil {
		return err
	}
	if err := validateHexString(signature.Sig); err != nil {
		return err
	}
	return nil
}

/*
validateSliceOfSignatures is a helper function used to validate multiple
signatures stored in a slice.
*/
func validateSliceOfSignatures(slice []Signature) error {
	for _, signature := range slice {
		if err := validateSignature(signature); err != nil {
			return err
		}
	}
	return nil
}

/*
Link represents the evidence of a supply chain step performed by a functionary.
It should be contained in a generic Metablock object, which provides
functionality for signing and signature verification, and reading from and
writing to disk.
*/
type Link struct {
	Type        string                 `json:"_type"`
	Name        string                 `json:"name"`
	Materials   map[string]interface{} `json:"materials"`
	Products    map[string]interface{} `json:"products"`
	ByProducts  map[string]interface{} `json:"byproducts"`
	Command     []string               `json:"command"`
	Environment map[string]interface{} `json:"environment"`
}

/*
validateArtifacts is a general function used to validate products and materials.
*/
func validateArtifacts(artifacts map[string]interface{}) error {
	for artifactName, artifact := range artifacts {
		artifactValue := reflect.ValueOf(artifact).MapRange()
		for artifactValue.Next() {
			value := artifactValue.Value().Interface().(string)
			hashType := artifactValue.Key().Interface().(string)
			if err := validateHexString(value); err != nil {
				return fmt.Errorf("in artifact '%s', %s hash value: %s",
					artifactName, hashType, err.Error())
			}
		}
	}
	return nil
}

/*
validateLink is a function used to ensure that a passed item of type Link
matches the necessary format.
*/
func validateLink(link Link) error {
	if link.Type != "link" {
		return fmt.Errorf("invalid type for link '%s': should be 'link'",
			link.Name)
	}

	if err := validateArtifacts(link.Materials); err != nil {
		return fmt.Errorf("in materials of link '%s': %s", link.Name,
			err.Error())
	}

	if err := validateArtifacts(link.Products); err != nil {
		return fmt.Errorf("in products of link '%s': %s", link.Name,
			err.Error())
	}

	return nil
}

/*
LinkNameFormat represents a format string used to create the filename for a
signed Link (wrapped in a Metablock). It consists of the name of the link and
the first 8 characters of the signing key id. E.g.:

	fmt.Sprintf(LinkNameFormat, "package",
	"2f89b9272acfc8f4a0a0f094d789fdb0ba798b0fe41f2f5f417c12f0085ff498")
	// returns "package.2f89b9272.link"
*/
const LinkNameFormat = "%s.%.8s.link"
const PreliminaryLinkNameFormat = ".%s.%.8s.link-unfinished"

/*
LinkNameFormatShort is for links that are not signed, e.g.:

	fmt.Sprintf(LinkNameFormatShort, "unsigned")
	// returns "unsigned.link"
*/
const LinkNameFormatShort = "%s.link"
const LinkGlobFormat = "%s.????????.link"

/*
SublayoutLinkDirFormat represents the format of the name of the directory for
sublayout links during the verification workflow.
*/
const SublayoutLinkDirFormat = "%s.%.8s"

/*
SupplyChainItem summarizes common fields of the two available supply chain
item types, Inspection and Step.
*/
type SupplyChainItem struct {
	Name              string     `json:"name"`
	ExpectedMaterials [][]string `json:"expected_materials"`
	ExpectedProducts  [][]string `json:"expected_products"`
}

/*
validateArtifactRule calls UnpackRule to validate that the passed rule conforms
with any of the available rule formats.
*/
func validateArtifactRule(rule []string) error {
	if _, err := UnpackRule(rule); err != nil {
		return err
	}
	return nil
}

/*
validateSliceOfArtifactRules iterates over passed rules to validate them.
*/
func validateSliceOfArtifactRules(rules [][]string) error {
	for _, rule := range rules {
		if err := validateArtifactRule(rule); err != nil {
			return err
		}
	}
	return nil
}

/*
validateSupplyChainItem is used to validate the common elements found in both
steps and inspections. Here, the function primarily ensures that the name of
a supply chain item isn't empty.
*/
func validateSupplyChainItem(item SupplyChainItem) error {
	if item.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if err := validateSliceOfArtifactRules(item.ExpectedMaterials); err != nil {
		return fmt.Errorf("invalid material rule: %s", err)
	}
	if err := validateSliceOfArtifactRules(item.ExpectedProducts); err != nil {
		return fmt.Errorf("invalid product rule: %s", err)
	}
	return nil
}

/*
Inspection represents an in-toto supply chain inspection, whose command in the
Run field is executed during final product verification, generating unsigned
link metadata.  Materials and products used/produced by the inspection are
constrained by the artifact rules in the inspection's ExpectedMaterials and
ExpectedProducts fields.
*/
type Inspection struct {
	Type string   `json:"_type"`
	Run  []string `json:"run"`
	SupplyChainItem
}

/*
validateInspection ensures that a passed inspection is valid and matches the
necessary format of an inspection.
*/
func validateInspection(inspection Inspection) error {
	if err := validateSupplyChainItem(inspection.SupplyChainItem); err != nil {
		return fmt.Errorf("inspection %s", err.Error())
	}
	if inspection.Type != "inspection" {
		return fmt.Errorf("invalid Type value for inspection '%s': should be "+
			"'inspection'", inspection.SupplyChainItem.Name)
	}
	return nil
}

/*
Step represents an in-toto step of the supply chain performed by a functionary.
During final product verification in-toto looks for corresponding Link
metadata, which is used as signed evidence that the step was performed
according to the supply chain definition.  Materials and products used/produced
by the step are constrained by the artifact rules in the step's
ExpectedMaterials and ExpectedProducts fields.
*/
type Step struct {
	Type                   string                  `json:"_type"`
	PubKeys                []string                `json:"pubkeys"`
	CertificateConstraints []CertificateConstraint `json:"cert_constraints,omitempty"`
	ExpectedCommand        []string                `json:"expected_command"`
	Threshold              int                     `json:"threshold"`
	SupplyChainItem
}

// CheckCertConstraints returns true if the provided certificate matches at least one
// of the constraints for this step.
func (s Step) CheckCertConstraints(key Key, rootCAIDs []string, rootCertPool, intermediateCertPool *x509.CertPool) error {
	if len(s.CertificateConstraints) == 0 {
		return fmt.Errorf("no constraints found")
	}

	_, possibleCert, err := decodeAndParse([]byte(key.KeyVal.Certificate))
	if err != nil {
		return err
	}

	cert, ok := possibleCert.(*x509.Certificate)
	if !ok {
		return fmt.Errorf("not a valid certificate")
	}

	for _, constraint := range s.CertificateConstraints {
		err = constraint.Check(cert, rootCAIDs, rootCertPool, intermediateCertPool)
		if err == nil {
			return nil
		}
	}
	if err != nil {
		return err
	}

	// this should not be reachable since there is at least one constraint, and the for loop only saw err != nil
	return fmt.Errorf("unknown certificate constraint error")
}

/*
validateStep ensures that a passed step is valid and matches the
necessary format of an step.
*/
func validateStep(step Step) error {
	if err := validateSupplyChainItem(step.SupplyChainItem); err != nil {
		return fmt.Errorf("step %s", err.Error())
	}
	if step.Type != "step" {
		return fmt.Errorf("invalid Type value for step '%s': should be 'step'",
			step.SupplyChainItem.Name)
	}
	for _, keyID := range step.PubKeys {
		if err := validateHexString(keyID); err != nil {
			return err
		}
	}
	return nil
}

/*
ISO8601DateSchema defines the format string of a timestamp following the
ISO 8601 standard.
*/
const ISO8601DateSchema = "2006-01-02T15:04:05Z"

/*
Layout represents the definition of a software supply chain.  It lists the
sequence of steps required in the software supply chain and the functionaries
authorized to perform these steps.  Functionaries are identified by their
public keys.  In addition, the layout may list a sequence of inspections that
are executed during in-toto supply chain verification.  A layout should be
contained in a generic Metablock object, which provides functionality for
signing and signature verification, and reading from and writing to disk.
*/
type Layout struct {
	Type            string         `json:"_type"`
	Steps           []Step         `json:"steps"`
	Inspect         []Inspection   `json:"inspect"`
	Keys            map[string]Key `json:"keys"`
	RootCas         map[string]Key `json:"rootcas,omitempty"`
	IntermediateCas map[string]Key `json:"intermediatecas,omitempty"`
	Expires         string         `json:"expires"`
	Readme          string         `json:"readme"`
}

// Go does not allow to pass `[]T` (slice with certain type) to a function
// that accepts `[]interface{}` (slice with generic type)
// We have to manually create the interface slice first, see
// https://golang.org/doc/faq#convert_slice_of_interface
// TODO: Is there a better way to do polymorphism for steps and inspections?
func (l *Layout) stepsAsInterfaceSlice() []interface{} {
	stepsI := make([]interface{}, len(l.Steps))
	for i, v := range l.Steps {
		stepsI[i] = v
	}
	return stepsI
}
func (l *Layout) inspectAsInterfaceSlice() []interface{} {
	inspectionsI := make([]interface{}, len(l.Inspect))
	for i, v := range l.Inspect {
		inspectionsI[i] = v
	}
	return inspectionsI
}

// RootCAIDs returns a slice of all of the Root CA IDs
func (l *Layout) RootCAIDs() []string {
	rootCAIDs := make([]string, 0, len(l.RootCas))
	for rootCAID := range l.RootCas {
		rootCAIDs = append(rootCAIDs, rootCAID)
	}
	return rootCAIDs
}

func validateLayoutKeys(keys map[string]Key) error {
	for keyID, key := range keys {
		if key.KeyID != keyID {
			return fmt.Errorf("invalid key found")
		}
		err := validatePublicKey(key)
		if err != nil {
			return err
		}
	}

	return nil
}

/*
validateLayout is a function used to ensure that a passed item of type Layout
matches the necessary format.
*/
func validateLayout(layout Layout) error {
	if layout.Type != "layout" {
		return fmt.Errorf("invalid Type value for layout: should be 'layout'")
	}

	if _, err := time.Parse(ISO8601DateSchema, layout.Expires); err != nil {
		return fmt.Errorf("expiry time parsed incorrectly - date either" +
			" invalid or of incorrect format")
	}

	if err := validateLayoutKeys(layout.Keys); err != nil {
		return err
	}

	if err := validateLayoutKeys(layout.RootCas); err != nil {
		return err
	}

	if err := validateLayoutKeys(layout.IntermediateCas); err != nil {
		return err
	}

	var namesSeen = make(map[string]bool)
	for _, step := range layout.Steps {
		if namesSeen[step.Name] {
			return fmt.Errorf("non unique step or inspection name found")
		}

		namesSeen[step.Name] = true

		if err := validateStep(step); err != nil {
			return err
		}
	}
	for _, inspection := range layout.Inspect {
		if namesSeen[inspection.Name] {
			return fmt.Errorf("non unique step or inspection name found")
		}

		namesSeen[inspection.Name] = true
	}
	return nil
}

type Metadata interface {
	Sign(Key) error
	VerifySignature(Key) error
	GetPayload() any
	Sigs() []Signature
	GetSignatureForKeyID(string) (Signature, error)
	Dump(string) error
}

func LoadMetadata(path string) (Metadata, error) {
	jsonBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var rawData map[string]*json.RawMessage
	if err := json.Unmarshal(jsonBytes, &rawData); err != nil {
		return nil, err
	}

	if _, ok := rawData["payloadType"]; ok {
		dsseEnv := &dsse.Envelope{}
		if rawData["payload"] == nil || rawData["signatures"] == nil {
			return nil, fmt.Errorf("in-toto metadata envelope requires 'payload' and 'signatures' parts")
		}

		if err := json.Unmarshal(jsonBytes, dsseEnv); err != nil {
			return nil, err
		}

		if dsseEnv.PayloadType != PayloadType {
			return nil, ErrInvalidPayloadType
		}

		return loadEnvelope(dsseEnv)
	}

	mb := &Metablock{}

	// Error out on missing `signed` or `signatures` field or if
	// one of them has a `null` value, which would lead to a nil pointer
	// dereference in Unmarshal below.
	if rawData["signed"] == nil || rawData["signatures"] == nil {
		return nil, fmt.Errorf("in-toto metadata requires 'signed' and 'signatures' parts")
	}

	// Fully unmarshal signatures part
	if err := json.Unmarshal(*rawData["signatures"], &mb.Signatures); err != nil {
		return nil, err
	}

	payload, err := loadPayload(*rawData["signed"])
	if err != nil {
		return nil, err
	}

	mb.Signed = payload

	return mb, nil
}

/*
Metablock is a generic container for signable in-toto objects such as Layout
or Link.  It has two fields, one that contains the signable object and one that
contains corresponding signatures.  Metablock also provides functionality for
signing and signature verification, and reading from and writing to disk.
*/
type Metablock struct {
	// NOTE: Whenever we want to access an attribute of `Signed` we have to
	// perform type assertion, e.g. `metablock.Signed.(Layout).Keys`
	// Maybe there is a better way to store either Layouts or Links in `Signed`?
	// The notary folks seem to have separate container structs:
	// https://github.com/theupdateframework/notary/blob/master/tuf/data/root.go#L10-L14
	// https://github.com/theupdateframework/notary/blob/master/tuf/data/targets.go#L13-L17
	// I implemented it this way, because there will be several functions that
	// receive or return a Metablock, where the type of Signed has to be inferred
	// on runtime, e.g. when iterating over links for a layout, and a link can
	// turn out to be a layout (sublayout)
	Signed     interface{} `json:"signed"`
	Signatures []Signature `json:"signatures"`
}

type jsonField struct {
	name      string
	omitempty bool
}

/*
checkRequiredJSONFields checks that the passed map (obj) has keys for each of
the json tags in the passed struct type (typ), and returns an error otherwise.
Any json tags that contain the "omitempty" option be allowed to be optional.
*/
func checkRequiredJSONFields(obj map[string]interface{},
	typ reflect.Type) error {

	// Create list of json tags, e.g. `json:"_type"`
	attributeCount := typ.NumField()
	allFields := make([]jsonField, 0)
	for i := 0; i < attributeCount; i++ {
		fieldStr := typ.Field(i).Tag.Get("json")
		field := jsonField{
			name:      fieldStr,
			omitempty: false,
		}

		if idx := strings.Index(fieldStr, ","); idx != -1 {
			field.name = fieldStr[:idx]
			field.omitempty = strings.Contains(fieldStr[idx+1:], "omitempty")
		}

		allFields = append(allFields, field)
	}

	// Assert that there's a key in the passed map for each tag
	for _, field := range allFields {
		if _, ok := obj[field.name]; !ok && !field.omitempty {
			return fmt.Errorf("required field %s missing", field.name)
		}
	}
	return nil
}

/*
Load parses JSON formatted metadata at the passed path into the Metablock
object on which it was called.  It returns an error if it cannot parse
a valid JSON formatted Metablock that contains a Link or Layout.

Deprecated: Use LoadMetadata for a signature wrapper agnostic way to load an
envelope.
*/
func (mb *Metablock) Load(path string) error {
	// Read entire file
	jsonBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Unmarshal JSON into a map of raw messages (signed and signatures)
	// We can't fully unmarshal immediately, because we need to inspect the
	// type (link or layout) to decide which data structure to use
	var rawMb map[string]*json.RawMessage
	if err := json.Unmarshal(jsonBytes, &rawMb); err != nil {
		return err
	}

	// Error out on missing `signed` or `signatures` field or if
	// one of them has a `null` value, which would lead to a nil pointer
	// dereference in Unmarshal below.
	if rawMb["signed"] == nil || rawMb["signatures"] == nil {
		return fmt.Errorf("in-toto metadata requires 'signed' and" +
			" 'signatures' parts")
	}

	// Fully unmarshal signatures part
	if err := json.Unmarshal(*rawMb["signatures"], &mb.Signatures); err != nil {
		return err
	}

	payload, err := loadPayload(*rawMb["signed"])
	if err != nil {
		return err
	}

	mb.Signed = payload

	return nil
}

/*
Dump JSON serializes and writes the Metablock on which it was called to the
passed path.  It returns an error if JSON serialization or writing fails.
*/
func (mb *Metablock) Dump(path string) error {
	// JSON encode Metablock formatted with newlines and indentation
	// TODO: parametrize format
	jsonBytes, err := json.MarshalIndent(mb, "", "  ")
	if err != nil {
		return err
	}

	// Write JSON bytes to the passed path with permissions (-rw-r--r--)
	err = os.WriteFile(path, jsonBytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

/*
GetSignableRepresentation returns the canonical JSON representation of the
Signed field of the Metablock on which it was called.  If canonicalization
fails the first return value is nil and the second return value is the error.
*/
func (mb *Metablock) GetSignableRepresentation() ([]byte, error) {
	return cjson.EncodeCanonical(mb.Signed)
}

func (mb *Metablock) GetPayload() any {
	return mb.Signed
}

func (mb *Metablock) Sigs() []Signature {
	return mb.Signatures
}

/*
VerifySignature verifies the first signature, corresponding to the passed Key,
that it finds in the Signatures field of the Metablock on which it was called.
It returns an error if Signatures does not contain a Signature corresponding to
the passed Key, the object in Signed cannot be canonicalized, or the Signature
is invalid.
*/
func (mb *Metablock) VerifySignature(key Key) error {
	sig, err := mb.GetSignatureForKeyID(key.KeyID)
	if err != nil {
		return err
	}

	dataCanonical, err := mb.GetSignableRepresentation()
	if err != nil {
		return err
	}

	if err := VerifySignature(key, sig, dataCanonical); err != nil {
		return err
	}
	return nil
}

// GetSignatureForKeyID returns the signature that was created by the provided keyID, if it exists.
func (mb *Metablock) GetSignatureForKeyID(keyID string) (Signature, error) {
	for _, s := range mb.Signatures {
		if s.KeyID == keyID {
			return s, nil
		}
	}

	return Signature{}, fmt.Errorf("no signature found for key '%s'", keyID)
}

/*
ValidateMetablock ensures that a passed Metablock object is valid. It indirectly
validates the Link or Layout that the Metablock object contains.
*/
func ValidateMetablock(mb Metablock) error {
	switch mbSignedType := mb.Signed.(type) {
	case Layout:
		if err := validateLayout(mb.Signed.(Layout)); err != nil {
			return err
		}
	case Link:
		if err := validateLink(mb.Signed.(Link)); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown type '%s', should be 'layout' or 'link'",
			mbSignedType)
	}

	if err := validateSliceOfSignatures(mb.Signatures); err != nil {
		return err
	}

	return nil
}

/*
Sign creates a signature over the signed portion of the metablock using the Key
object provided. It then appends the resulting signature to the signatures
field as provided. It returns an error if the Signed object cannot be
canonicalized, or if the key is invalid or not supported.
*/
func (mb *Metablock) Sign(key Key) error {

	dataCanonical, err := mb.GetSignableRepresentation()
	if err != nil {
		return err
	}

	newSignature, err := GenerateSignature(dataCanonical, key)
	if err != nil {
		return err
	}

	mb.Signatures = append(mb.Signatures, newSignature)
	return nil
}
