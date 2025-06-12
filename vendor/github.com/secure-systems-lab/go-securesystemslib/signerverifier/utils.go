package signerverifier

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"hash"
	"testing"

	"github.com/secure-systems-lab/go-securesystemslib/cjson"
)

/*
Credits: Parts of this file were originally authored for in-toto-golang.
*/

var (
	// ErrNoPEMBlock gets triggered when there is no PEM block in the provided file
	ErrNoPEMBlock = errors.New("failed to decode the data as PEM block (are you sure this is a pem file?)")
	// ErrFailedPEMParsing gets returned when PKCS1, PKCS8 or PKIX key parsing fails
	ErrFailedPEMParsing = errors.New("failed parsing the PEM block: unsupported PEM type")
)

// loadKeyFromSSLibBytes returns a pointer to a Key instance created from the
// contents of the bytes. The key contents are expected to be in the custom
// securesystemslib format.
func loadKeyFromSSLibBytes(contents []byte) (*SSLibKey, error) {
	var key *SSLibKey
	if err := json.Unmarshal(contents, &key); err != nil {
		return nil, err
	}

	if len(key.KeyID) == 0 {
		keyID, err := calculateKeyID(key)
		if err != nil {
			return nil, err
		}
		key.KeyID = keyID
	}

	return key, nil
}

func calculateKeyID(k *SSLibKey) (string, error) {
	key := map[string]any{
		"keytype":               k.KeyType,
		"scheme":                k.Scheme,
		"keyid_hash_algorithms": k.KeyIDHashAlgorithms,
		"keyval": map[string]string{
			"public": k.KeyVal.Public,
		},
	}
	canonical, err := cjson.EncodeCanonical(key)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

/*
generatePEMBlock creates a PEM block from scratch via the keyBytes and the pemType.
If successful it returns a PEM block as []byte slice. This function should always
succeed, if keyBytes is empty the PEM block will have an empty byte block.
Therefore only header and footer will exist.
*/
func generatePEMBlock(keyBytes []byte, pemType string) []byte {
	// construct PEM block
	pemBlock := &pem.Block{
		Type:    pemType,
		Headers: nil,
		Bytes:   keyBytes,
	}
	return pem.EncodeToMemory(pemBlock)
}

/*
decodeAndParsePEM receives potential PEM bytes decodes them via pem.Decode
and pushes them to parseKey. If any error occurs during this process,
the function will return nil and an error (either ErrFailedPEMParsing
or ErrNoPEMBlock). On success it will return the decoded pemData, the
key object interface and nil as error. We need the decoded pemData,
because LoadKey relies on decoded pemData for operating system
interoperability.
*/
func decodeAndParsePEM(pemBytes []byte) (*pem.Block, any, error) {
	// pem.Decode returns the parsed pem block and a rest.
	// The rest is everything, that could not be parsed as PEM block.
	// Therefore we can drop this via using the blank identifier "_"
	data, _ := pem.Decode(pemBytes)
	if data == nil {
		return nil, nil, ErrNoPEMBlock
	}

	// Try to load private key, if this fails try to load
	// key as public key
	key, err := parsePEMKey(data.Bytes)
	if err != nil {
		return nil, nil, err
	}
	return data, key, nil
}

/*
parseKey tries to parse a PEM []byte slice. Using the following standards
in the given order:

  - PKCS8
  - PKCS1
  - PKIX

On success it returns the parsed key and nil.
On failure it returns nil and the error ErrFailedPEMParsing
*/
func parsePEMKey(data []byte) (any, error) {
	key, err := x509.ParsePKCS8PrivateKey(data)
	if err == nil {
		return key, nil
	}
	key, err = x509.ParsePKCS1PrivateKey(data)
	if err == nil {
		return key, nil
	}
	key, err = x509.ParsePKIXPublicKey(data)
	if err == nil {
		return key, nil
	}
	key, err = x509.ParseECPrivateKey(data)
	if err == nil {
		return key, nil
	}
	return nil, ErrFailedPEMParsing
}

func hashBeforeSigning(data []byte, h hash.Hash) []byte {
	h.Write(data)
	return h.Sum(nil)
}

func hexDecode(t *testing.T, data string) []byte {
	t.Helper()
	b, err := hex.DecodeString(data)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
