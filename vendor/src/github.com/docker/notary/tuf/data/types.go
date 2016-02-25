package data

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/go/canonical/json"
	"github.com/docker/notary"
)

// SigAlgorithm for types of signatures
type SigAlgorithm string

func (k SigAlgorithm) String() string {
	return string(k)
}

const defaultHashAlgorithm = "sha256"

// Signature types
const (
	EDDSASignature       SigAlgorithm = "eddsa"
	RSAPSSSignature      SigAlgorithm = "rsapss"
	RSAPKCS1v15Signature SigAlgorithm = "rsapkcs1v15"
	ECDSASignature       SigAlgorithm = "ecdsa"
	PyCryptoSignature    SigAlgorithm = "pycrypto-pkcs#1 pss"
)

// Key types
const (
	ED25519Key   = "ed25519"
	RSAKey       = "rsa"
	RSAx509Key   = "rsa-x509"
	ECDSAKey     = "ecdsa"
	ECDSAx509Key = "ecdsa-x509"
)

// TUFTypes is the set of metadata types
var TUFTypes = map[string]string{
	CanonicalRootRole:      "Root",
	CanonicalTargetsRole:   "Targets",
	CanonicalSnapshotRole:  "Snapshot",
	CanonicalTimestampRole: "Timestamp",
}

// SetTUFTypes allows one to override some or all of the default
// type names in TUF.
func SetTUFTypes(ts map[string]string) {
	for k, v := range ts {
		TUFTypes[k] = v
	}
}

// ValidTUFType checks if the given type is valid for the role
func ValidTUFType(typ, role string) bool {
	if ValidRole(role) {
		// All targets delegation roles must have
		// the valid type is for targets.
		if role == "" {
			// role is unknown and does not map to
			// a type
			return false
		}
		if strings.HasPrefix(role, CanonicalTargetsRole+"/") {
			role = CanonicalTargetsRole
		}
	}
	// most people will just use the defaults so have this optimal check
	// first. Do comparison just in case there is some unknown vulnerability
	// if a key and value in the map differ.
	if v, ok := TUFTypes[role]; ok {
		return typ == v
	}
	return false
}

// Signed is the high level, partially deserialized metadata object
// used to verify signatures before fully unpacking, or to add signatures
// before fully packing
type Signed struct {
	Signed     json.RawMessage `json:"signed"`
	Signatures []Signature     `json:"signatures"`
}

// SignedCommon contains the fields common to the Signed component of all
// TUF metadata files
type SignedCommon struct {
	Type    string    `json:"_type"`
	Expires time.Time `json:"expires"`
	Version int       `json:"version"`
}

// SignedMeta is used in server validation where we only need signatures
// and common fields
type SignedMeta struct {
	Signed     SignedCommon `json:"signed"`
	Signatures []Signature  `json:"signatures"`
}

// Signature is a signature on a piece of metadata
type Signature struct {
	KeyID     string       `json:"keyid"`
	Method    SigAlgorithm `json:"method"`
	Signature []byte       `json:"sig"`
}

// Files is the map of paths to file meta container in targets and delegations
// metadata files
type Files map[string]FileMeta

// Hashes is the map of hash type to digest created for each metadata
// and target file
type Hashes map[string][]byte

// FileMeta contains the size and hashes for a metadata or target file. Custom
// data can be optionally added.
type FileMeta struct {
	Length int64           `json:"length"`
	Hashes Hashes          `json:"hashes"`
	Custom json.RawMessage `json:"custom,omitempty"`
}

// NewFileMeta generates a FileMeta object from the reader, using the
// hash algorithms provided
func NewFileMeta(r io.Reader, hashAlgorithms ...string) (FileMeta, error) {
	if len(hashAlgorithms) == 0 {
		hashAlgorithms = []string{defaultHashAlgorithm}
	}
	hashes := make(map[string]hash.Hash, len(hashAlgorithms))
	for _, hashAlgorithm := range hashAlgorithms {
		var h hash.Hash
		switch hashAlgorithm {
		case "sha256":
			h = sha256.New()
		case "sha512":
			h = sha512.New()
		default:
			return FileMeta{}, fmt.Errorf("Unknown Hash Algorithm: %s", hashAlgorithm)
		}
		hashes[hashAlgorithm] = h
		r = io.TeeReader(r, h)
	}
	n, err := io.Copy(ioutil.Discard, r)
	if err != nil {
		return FileMeta{}, err
	}
	m := FileMeta{Length: n, Hashes: make(Hashes, len(hashes))}
	for hashAlgorithm, h := range hashes {
		m.Hashes[hashAlgorithm] = h.Sum(nil)
	}
	return m, nil
}

// Delegations holds a tier of targets delegations
type Delegations struct {
	Keys  Keys    `json:"keys"`
	Roles []*Role `json:"roles"`
}

// NewDelegations initializes an empty Delegations object
func NewDelegations() *Delegations {
	return &Delegations{
		Keys:  make(map[string]PublicKey),
		Roles: make([]*Role, 0),
	}
}

// These values are recommended TUF expiry times.
var defaultExpiryTimes = map[string]time.Duration{
	CanonicalRootRole:      notary.Year,
	CanonicalTargetsRole:   90 * notary.Day,
	CanonicalSnapshotRole:  7 * notary.Day,
	CanonicalTimestampRole: notary.Day,
}

// SetDefaultExpiryTimes allows one to change the default expiries.
func SetDefaultExpiryTimes(times map[string]time.Duration) {
	for key, value := range times {
		if _, ok := defaultExpiryTimes[key]; !ok {
			logrus.Errorf("Attempted to set default expiry for an unknown role: %s", key)
			continue
		}
		defaultExpiryTimes[key] = value
	}
}

// DefaultExpires gets the default expiry time for the given role
func DefaultExpires(role string) time.Time {
	if d, ok := defaultExpiryTimes[role]; ok {
		return time.Now().Add(d)
	}
	var t time.Time
	return t.UTC().Round(time.Second)
}

type unmarshalledSignature Signature

// UnmarshalJSON does a custom unmarshalling of the signature JSON
func (s *Signature) UnmarshalJSON(data []byte) error {
	uSignature := unmarshalledSignature{}
	err := json.Unmarshal(data, &uSignature)
	if err != nil {
		return err
	}
	uSignature.Method = SigAlgorithm(strings.ToLower(string(uSignature.Method)))
	*s = Signature(uSignature)
	return nil
}
