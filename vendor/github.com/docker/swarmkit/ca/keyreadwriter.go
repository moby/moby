package ca

import (
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"crypto/tls"

	"github.com/docker/swarmkit/ca/keyutils"
	"github.com/docker/swarmkit/ca/pkcs8"
	"github.com/docker/swarmkit/ioutils"
	"github.com/pkg/errors"
)

const (
	// keyPerms are the permissions used to write the TLS keys
	keyPerms = 0600
	// certPerms are the permissions used to write TLS certificates
	certPerms = 0644
	// versionHeader is the TLS PEM key header that contains the KEK version
	versionHeader = "kek-version"
)

// PEMKeyHeaders is an interface for something that needs to know about PEM headers
// when reading or writing TLS keys in order to keep them updated with the latest
// KEK.
type PEMKeyHeaders interface {

	// UnmarshalHeaders loads the headers map given the current KEK
	UnmarshalHeaders(map[string]string, KEKData) (PEMKeyHeaders, error)

	// MarshalHeaders returns a header map given the current KEK
	MarshalHeaders(KEKData) (map[string]string, error)

	// UpdateKEK gets called whenever KeyReadWriter gets a KEK update.  This allows the
	// PEMKeyHeaders to optionally update any internal state.  It should return an
	// updated (if needed) versino of itself.
	UpdateKEK(KEKData, KEKData) PEMKeyHeaders
}

// KeyReader reads a TLS cert and key from disk
type KeyReader interface {

	// Read reads and returns the certificate and the key PEM bytes that are on disk
	Read() ([]byte, []byte, error)

	// Target returns a string representation of where the cert data is being read from
	Target() string
}

// KeyWriter writes a TLS key and cert to disk
type KeyWriter interface {

	// Write accepts a certificate and key in PEM format, as well as an optional KEKData object.
	// If there is a current KEK, the key is encrypted using the current KEK.  If the KEKData object
	// is provided (not nil), the key will be encrypted using the new KEK instead, and the current
	// KEK in memory will be replaced by the provided KEKData.  The reason to allow changing key
	// material and KEK in a single step, as opposed to two steps, is to prevent the key material
	// from being written unencrypted or with an old KEK in the first place (when a node gets a
	// certificate from the CA, it will also request the current KEK so it won't have to immediately
	// do a KEK rotation after getting the key).
	Write([]byte, []byte, *KEKData) error

	// ViewAndUpdateHeaders is a function that reads and updates the headers of the key in a single
	// transaction (e.g. within a lock).  It accepts a callback function which will be passed the
	// current header management object, and which must return a new, updated, or same header
	// management object.  KeyReadWriter then performs the following actions:
	//    - uses the old header management object and the current KEK to deserialize/decrypt
	//      the existing PEM headers
	//    - uses the new header management object and the current KEK to to reserialize/encrypt
	//      the PEM headers
	//    - writes the new PEM headers, as well as the key material, unchanged, to disk
	ViewAndUpdateHeaders(func(PEMKeyHeaders) (PEMKeyHeaders, error)) error

	// ViewAndRotateKEK is a function that just re-encrypts the TLS key and headers in a single
	// transaction (e.g. within a lock).  It accepts a callback unction which will be passed the
	// current KEK and the current headers management object, and which should return a new
	// KEK and header management object.  KeyReadWriter then performs the following actions:
	//    - uses the old KEK and header management object to deserialize/decrypt the
	//      TLS key and PEM headers
	//    - uses the new KEK and header management object to serialize/encrypt the TLS key
	//      and PEM headers
	//    - writes the new PEM headers and newly encrypted TLS key to disk
	ViewAndRotateKEK(func(KEKData, PEMKeyHeaders) (KEKData, PEMKeyHeaders, error)) error

	// GetCurrentState returns the current header management object and the current KEK.
	GetCurrentState() (PEMKeyHeaders, KEKData)

	// Target returns a string representation of where the cert data is being read from
	Target() string
}

// KEKData provides an optional update to the kek when writing.  The structure
// is needed so that we can tell the difference between "do not encrypt anymore"
// and there is "no update".
type KEKData struct {
	KEK     []byte
	Version uint64
}

// ErrInvalidKEK means that we cannot decrypt the TLS key for some reason
type ErrInvalidKEK struct {
	Wrapped error
}

func (e ErrInvalidKEK) Error() string {
	return e.Wrapped.Error()
}

// KeyReadWriter is an object that knows how to read and write TLS keys and certs to disk,
// optionally encrypted and optionally updating PEM headers.  It should be the only object which
// can write the TLS key, to ensure that writes are serialized and that the TLS key, the
// KEK (key encrypting key), and any headers which need to be written are never out of sync.
// It accepts a PEMKeyHeaders object, which is used to serialize/encrypt and deserialize/decrypt
// the PEM headers when given the current headers and the current KEK.
type KeyReadWriter struct {

	// This lock is held whenever a key is read from or written to disk, or whenever the internal
	// state of the KeyReadWriter (such as the KEK, the key formatter, or the PEM header management
	// object changes.)
	mu sync.Mutex

	kekData      KEKData
	paths        CertPaths
	headersObj   PEMKeyHeaders
	keyFormatter keyutils.Formatter
}

// NewKeyReadWriter creates a new KeyReadWriter
func NewKeyReadWriter(paths CertPaths, kek []byte, headersObj PEMKeyHeaders) *KeyReadWriter {
	return &KeyReadWriter{
		kekData:      KEKData{KEK: kek},
		paths:        paths,
		headersObj:   headersObj,
		keyFormatter: keyutils.Default,
	}
}

// SetKeyFormatter sets the keyformatter with which to encrypt and decrypt keys
func (k *KeyReadWriter) SetKeyFormatter(kf keyutils.Formatter) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.keyFormatter = kf
}

// Migrate checks to see if a temporary key file exists.  Older versions of
// swarmkit wrote temporary keys instead of temporary certificates, so
// migrate that temporary key if it exists.  We want to write temporary certificates,
// instead of temporary keys, because we may need to periodically re-encrypt the
// keys and modify the headers, and it's easier to have a single canonical key
// location than two possible key locations.
func (k *KeyReadWriter) Migrate() error {
	tmpPaths := k.genTempPaths()
	keyBytes, err := ioutil.ReadFile(tmpPaths.Key)
	if err != nil {
		return nil // no key?  no migration
	}

	// it does exist - no need to decrypt, because previous versions of swarmkit
	// which supported this temporary key did not support encrypting TLS keys
	cert, err := ioutil.ReadFile(k.paths.Cert)
	if err != nil {
		return os.RemoveAll(tmpPaths.Key) // no cert?  no migration
	}

	// nope, this does not match the cert
	if _, err = tls.X509KeyPair(cert, keyBytes); err != nil {
		return os.RemoveAll(tmpPaths.Key)
	}

	return os.Rename(tmpPaths.Key, k.paths.Key)
}

// Read will read a TLS cert and key from the given paths
func (k *KeyReadWriter) Read() ([]byte, []byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	keyBlock, err := k.readKey()
	if err != nil {
		return nil, nil, err
	}

	if version, ok := keyBlock.Headers[versionHeader]; ok {
		if versionInt, err := strconv.ParseUint(version, 10, 64); err == nil {
			k.kekData.Version = versionInt
		}
	}
	delete(keyBlock.Headers, versionHeader)

	if k.headersObj != nil {
		newHeaders, err := k.headersObj.UnmarshalHeaders(keyBlock.Headers, k.kekData)
		if err != nil {
			return nil, nil, errors.Wrap(err, "unable to read TLS key headers")
		}
		k.headersObj = newHeaders
	}

	keyBytes := pem.EncodeToMemory(keyBlock)
	cert, err := ioutil.ReadFile(k.paths.Cert)
	// The cert is written to a temporary file first, then the key, and then
	// the cert gets renamed - so, if interrupted, it's possible to end up with
	// a cert that only exists in the temporary location.
	switch {
	case err == nil:
		_, err = tls.X509KeyPair(cert, keyBytes)
	case os.IsNotExist(err): //continue to try temp location
		break
	default:
		return nil, nil, err
	}

	// either the cert doesn't exist, or it doesn't match the key - try the temp file, if it exists
	if err != nil {
		var tempErr error
		tmpPaths := k.genTempPaths()
		cert, tempErr = ioutil.ReadFile(tmpPaths.Cert)
		if tempErr != nil {
			return nil, nil, err // return the original error
		}
		if _, tempErr := tls.X509KeyPair(cert, keyBytes); tempErr != nil {
			os.RemoveAll(tmpPaths.Cert) // nope, it doesn't match either - remove and return the original error
			return nil, nil, err
		}
		os.Rename(tmpPaths.Cert, k.paths.Cert) // try to move the temp cert back to the regular location

	}

	return cert, keyBytes, nil
}

// ViewAndRotateKEK re-encrypts the key with a new KEK
func (k *KeyReadWriter) ViewAndRotateKEK(cb func(KEKData, PEMKeyHeaders) (KEKData, PEMKeyHeaders, error)) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	updatedKEK, updatedHeaderObj, err := cb(k.kekData, k.headersObj)
	if err != nil {
		return err
	}

	keyBlock, err := k.readKey()
	if err != nil {
		return err
	}

	return k.writeKey(keyBlock, updatedKEK, updatedHeaderObj)
}

// ViewAndUpdateHeaders updates the header manager, and updates any headers on the existing key
func (k *KeyReadWriter) ViewAndUpdateHeaders(cb func(PEMKeyHeaders) (PEMKeyHeaders, error)) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	pkh, err := cb(k.headersObj)
	if err != nil {
		return err
	}

	keyBlock, err := k.readKeyblock()
	if err != nil {
		return err
	}

	headers := make(map[string]string)
	if pkh != nil {
		var err error
		headers, err = pkh.MarshalHeaders(k.kekData)
		if err != nil {
			return err
		}
	}
	// we WANT any original encryption headers
	for key, value := range keyBlock.Headers {
		normalizedKey := strings.TrimSpace(strings.ToLower(key))
		if normalizedKey == "proc-type" || normalizedKey == "dek-info" {
			headers[key] = value
		}
	}
	headers[versionHeader] = strconv.FormatUint(k.kekData.Version, 10)
	keyBlock.Headers = headers

	if err = ioutils.AtomicWriteFile(k.paths.Key, pem.EncodeToMemory(keyBlock), keyPerms); err != nil {
		return err
	}
	k.headersObj = pkh
	return nil
}

// GetCurrentState returns the current KEK data, including version
func (k *KeyReadWriter) GetCurrentState() (PEMKeyHeaders, KEKData) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.headersObj, k.kekData
}

// Write attempts write a cert and key to text.  This can also optionally update
// the KEK while writing, if an updated KEK is provided.  If the pointer to the
// update KEK is nil, then we don't update. If the updated KEK itself is nil,
// then we update the KEK to be nil (data should be unencrypted).
func (k *KeyReadWriter) Write(certBytes, plaintextKeyBytes []byte, kekData *KEKData) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	// current assumption is that the cert and key will be in the same directory
	if err := os.MkdirAll(filepath.Dir(k.paths.Key), 0755); err != nil {
		return err
	}

	// Ensure that we will have a keypair on disk at all times by writing the cert to a
	// temp path first.  This is because we want to have only a single copy of the key
	// for rotation and header modification.
	tmpPaths := k.genTempPaths()
	if err := ioutils.AtomicWriteFile(tmpPaths.Cert, certBytes, certPerms); err != nil {
		return err
	}

	keyBlock, _ := pem.Decode(plaintextKeyBytes)
	if keyBlock == nil {
		return errors.New("invalid PEM-encoded private key")
	}

	if kekData == nil {
		kekData = &k.kekData
	}
	pkh := k.headersObj
	if k.headersObj != nil {
		pkh = k.headersObj.UpdateKEK(k.kekData, *kekData)
	}

	if err := k.writeKey(keyBlock, *kekData, pkh); err != nil {
		return err
	}
	return os.Rename(tmpPaths.Cert, k.paths.Cert)
}

func (k *KeyReadWriter) genTempPaths() CertPaths {
	return CertPaths{
		Key:  filepath.Join(filepath.Dir(k.paths.Key), "."+filepath.Base(k.paths.Key)),
		Cert: filepath.Join(filepath.Dir(k.paths.Cert), "."+filepath.Base(k.paths.Cert)),
	}
}

// Target returns a string representation of this KeyReadWriter, namely where
// it is writing to
func (k *KeyReadWriter) Target() string {
	return k.paths.Cert
}

func (k *KeyReadWriter) readKeyblock() (*pem.Block, error) {
	key, err := ioutil.ReadFile(k.paths.Key)
	if err != nil {
		return nil, err
	}

	// Decode the PEM private key
	keyBlock, _ := pem.Decode(key)
	if keyBlock == nil {
		return nil, errors.New("invalid PEM-encoded private key")
	}

	return keyBlock, nil
}

// readKey returns the decrypted key pem bytes, and enforces the KEK if applicable
// (writes it back with the correct encryption if it is not correctly encrypted)
func (k *KeyReadWriter) readKey() (*pem.Block, error) {
	keyBlock, err := k.readKeyblock()
	if err != nil {
		return nil, err
	}

	if !keyutils.IsEncryptedPEMBlock(keyBlock) {
		return keyBlock, nil
	}

	// If it's encrypted, we can't read without a passphrase (we're assuming
	// empty passphrases are invalid)
	if k.kekData.KEK == nil {
		return nil, ErrInvalidKEK{Wrapped: x509.IncorrectPasswordError}
	}

	derBytes, err := k.keyFormatter.DecryptPEMBlock(keyBlock, k.kekData.KEK)
	if err == keyutils.ErrFIPSUnsupportedKeyFormat {
		return nil, err
	} else if err != nil {
		return nil, ErrInvalidKEK{Wrapped: err}
	}

	// change header only if its pkcs8
	if keyBlock.Type == "ENCRYPTED PRIVATE KEY" {
		keyBlock.Type = "PRIVATE KEY"
	}

	// remove encryption PEM headers
	headers := make(map[string]string)
	mergePEMHeaders(headers, keyBlock.Headers)

	return &pem.Block{
		Type:    keyBlock.Type, // the key type doesn't change
		Bytes:   derBytes,
		Headers: headers,
	}, nil
}

// writeKey takes an unencrypted keyblock and, if the kek is not nil, encrypts it before
// writing it to disk.  If the kek is nil, writes it to disk unencrypted.
func (k *KeyReadWriter) writeKey(keyBlock *pem.Block, kekData KEKData, pkh PEMKeyHeaders) error {
	if kekData.KEK != nil {
		encryptedPEMBlock, err := k.keyFormatter.EncryptPEMBlock(keyBlock.Bytes, kekData.KEK)
		if err != nil {
			return err
		}
		if !keyutils.IsEncryptedPEMBlock(encryptedPEMBlock) {
			return errors.New("unable to encrypt key - invalid PEM file produced")
		}
		keyBlock = encryptedPEMBlock
	}

	if pkh != nil {
		headers, err := pkh.MarshalHeaders(kekData)
		if err != nil {
			return err
		}
		mergePEMHeaders(keyBlock.Headers, headers)
	}
	keyBlock.Headers[versionHeader] = strconv.FormatUint(kekData.Version, 10)

	if err := ioutils.AtomicWriteFile(k.paths.Key, pem.EncodeToMemory(keyBlock), keyPerms); err != nil {
		return err
	}
	k.kekData = kekData
	k.headersObj = pkh
	return nil
}

// DowngradeKey converts the PKCS#8 key to PKCS#1 format and save it
func (k *KeyReadWriter) DowngradeKey() error {
	_, key, err := k.Read()
	if err != nil {
		return err
	}

	oldBlock, _ := pem.Decode(key)
	if oldBlock == nil {
		return errors.New("invalid PEM-encoded private key")
	}

	// stop if the key is already downgraded to pkcs1
	if !keyutils.IsPKCS8(oldBlock.Bytes) {
		return errors.New("key is already downgraded to PKCS#1")
	}

	eckey, err := pkcs8.ConvertToECPrivateKeyPEM(key)
	if err != nil {
		return err
	}

	newBlock, _ := pem.Decode(eckey)
	if newBlock == nil {
		return errors.New("invalid PEM-encoded private key")
	}

	if k.kekData.KEK != nil {
		newBlock, err = k.keyFormatter.EncryptPEMBlock(newBlock.Bytes, k.kekData.KEK)
		if err != nil {
			return err
		}
	}

	// add kek-version header back to the new key
	newBlock.Headers[versionHeader] = strconv.FormatUint(k.kekData.Version, 10)
	mergePEMHeaders(newBlock.Headers, oldBlock.Headers)

	// do not use krw.Write as it will convert the key to pkcs8
	return ioutils.AtomicWriteFile(k.paths.Key, pem.EncodeToMemory(newBlock), keyPerms)
}

// merges one set of PEM headers onto another, excepting for key encryption value
// "proc-type" and "dek-info"
func mergePEMHeaders(original, newSet map[string]string) {
	for key, value := range newSet {
		normalizedKey := strings.TrimSpace(strings.ToLower(key))
		if normalizedKey != "proc-type" && normalizedKey != "dek-info" {
			original[key] = value
		}
	}
}
