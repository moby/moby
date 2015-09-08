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
	"github.com/jfrazelle/go/canonical/json"
)

type KeyAlgorithm string

func (k KeyAlgorithm) String() string {
	return string(k)
}

type SigAlgorithm string

func (k SigAlgorithm) String() string {
	return string(k)
}

const (
	defaultHashAlgorithm = "sha256"

	EDDSASignature       SigAlgorithm = "eddsa"
	RSAPSSSignature      SigAlgorithm = "rsapss"
	RSAPKCS1v15Signature SigAlgorithm = "rsapkcs1v15"
	ECDSASignature       SigAlgorithm = "ecdsa"
	PyCryptoSignature    SigAlgorithm = "pycrypto-pkcs#1 pss"

	ED25519Key   KeyAlgorithm = "ed25519"
	RSAKey       KeyAlgorithm = "rsa"
	RSAx509Key   KeyAlgorithm = "rsa-x509"
	ECDSAKey     KeyAlgorithm = "ecdsa"
	ECDSAx509Key KeyAlgorithm = "ecdsa-x509"
)

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

func ValidTUFType(typ, role string) bool {
	if ValidRole(role) {
		// All targets delegation roles must have
		// the valid type is for targets.
		role = CanonicalRole(role)
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

type Signed struct {
	Signed     json.RawMessage `json:"signed"`
	Signatures []Signature     `json:"signatures"`
}

type SignedCommon struct {
	Type    string    `json:"_type"`
	Expires time.Time `json:"expires"`
	Version int       `json:"version"`
}

type SignedMeta struct {
	Signed     SignedCommon `json:"signed"`
	Signatures []Signature  `json:"signatures"`
}

type Signature struct {
	KeyID     string       `json:"keyid"`
	Method    SigAlgorithm `json:"method"`
	Signature []byte       `json:"sig"`
}

type Files map[string]FileMeta

type Hashes map[string][]byte

type FileMeta struct {
	Length int64           `json:"length"`
	Hashes Hashes          `json:"hashes"`
	Custom json.RawMessage `json:"custom,omitempty"`
}

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

type Delegations struct {
	Keys  map[string]PublicKey `json:"keys"`
	Roles []*Role              `json:"roles"`
}

func NewDelegations() *Delegations {
	return &Delegations{
		Keys:  make(map[string]PublicKey),
		Roles: make([]*Role, 0),
	}
}

// defines number of days in which something should expire
var defaultExpiryTimes = map[string]int{
	CanonicalRootRole:      365,
	CanonicalTargetsRole:   90,
	CanonicalSnapshotRole:  7,
	CanonicalTimestampRole: 1,
}

// SetDefaultExpiryTimes allows one to change the default expiries.
func SetDefaultExpiryTimes(times map[string]int) {
	for key, value := range times {
		if _, ok := defaultExpiryTimes[key]; !ok {
			logrus.Errorf("Attempted to set default expiry for an unknown role: %s", key)
			continue
		}
		defaultExpiryTimes[key] = value
	}
}

func DefaultExpires(role string) time.Time {
	var t time.Time
	if t, ok := defaultExpiryTimes[role]; ok {
		return time.Now().AddDate(0, 0, t)
	}
	return t.UTC().Round(time.Second)
}

type unmarshalledSignature Signature

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
