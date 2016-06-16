// +build pkcs11

package yubikey

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary/passphrase"
	"github.com/docker/notary/trustmanager"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/signed"
	"github.com/miekg/pkcs11"
)

const (
	// UserPin is the user pin of a yubikey (in PIV parlance, is the PIN)
	UserPin = "123456"
	// SOUserPin is the "Security Officer" user pin - this is the PIV management
	// (MGM) key, which is different than the admin pin of the Yubikey PGP interface
	// (which in PIV parlance is the PUK, and defaults to 12345678)
	SOUserPin = "010203040506070801020304050607080102030405060708"
	numSlots  = 4 // number of slots in the yubikey

	// KeymodeNone means that no touch or PIN is required to sign with the yubikey
	KeymodeNone = 0
	// KeymodeTouch means that only touch is required to sign with the yubikey
	KeymodeTouch = 1
	// KeymodePinOnce means that the pin entry is required once the first time to sign with the yubikey
	KeymodePinOnce = 2
	// KeymodePinAlways means that pin entry is required every time to sign with the yubikey
	KeymodePinAlways = 4

	// the key size, when importing a key into yubikey, MUST be 32 bytes
	ecdsaPrivateKeySize = 32

	sigAttempts = 5
)

// what key mode to use when generating keys
var (
	yubikeyKeymode = KeymodeTouch | KeymodePinOnce
	// order in which to prefer token locations on the yubikey.
	// corresponds to: 9c, 9e, 9d, 9a
	slotIDs = []int{2, 1, 3, 0}
)

// SetYubikeyKeyMode - sets the mode when generating yubikey keys.
// This is to be used for testing.  It does nothing if not building with tag
// pkcs11.
func SetYubikeyKeyMode(keyMode int) error {
	// technically 7 (1 | 2 | 4) is valid, but KeymodePinOnce +
	// KeymdoePinAlways don't really make sense together
	if keyMode < 0 || keyMode > 5 {
		return errors.New("Invalid key mode")
	}
	yubikeyKeymode = keyMode
	return nil
}

// SetTouchToSignUI - allows configurable UX for notifying a user that they
// need to touch the yubikey to sign. The callback may be used to provide a
// mechanism for updating a GUI (such as removing a modal) after the touch
// has been made
func SetTouchToSignUI(notifier func(), callback func()) {
	touchToSignUI = notifier
	if callback != nil {
		touchDoneCallback = callback
	}
}

var touchToSignUI = func() {
	fmt.Println("Please touch the attached Yubikey to perform signing.")
}

var touchDoneCallback = func() {
	// noop
}

var pkcs11Lib string

func init() {
	for _, loc := range possiblePkcs11Libs {
		_, err := os.Stat(loc)
		if err == nil {
			p := pkcs11.New(loc)
			if p != nil {
				pkcs11Lib = loc
				return
			}
		}
	}
}

// ErrBackupFailed is returned when a YubiStore fails to back up a key that
// is added
type ErrBackupFailed struct {
	err string
}

func (err ErrBackupFailed) Error() string {
	return fmt.Sprintf("Failed to backup private key to: %s", err.err)
}

// An error indicating that the HSM is not present (as opposed to failing),
// i.e. that we can confidently claim that the key is not stored in the HSM
// without notifying the user about a missing or failing HSM.
type errHSMNotPresent struct {
	err string
}

func (err errHSMNotPresent) Error() string {
	return err.err
}

type yubiSlot struct {
	role   string
	slotID []byte
}

// YubiPrivateKey represents a private key inside of a yubikey
type YubiPrivateKey struct {
	data.ECDSAPublicKey
	passRetriever passphrase.Retriever
	slot          []byte
	libLoader     pkcs11LibLoader
}

// yubikeySigner wraps a YubiPrivateKey and implements the crypto.Signer interface
type yubikeySigner struct {
	YubiPrivateKey
}

// NewYubiPrivateKey returns a YubiPrivateKey, which implements the data.PrivateKey
// interface except that the private material is inacessible
func NewYubiPrivateKey(slot []byte, pubKey data.ECDSAPublicKey,
	passRetriever passphrase.Retriever) *YubiPrivateKey {

	return &YubiPrivateKey{
		ECDSAPublicKey: pubKey,
		passRetriever:  passRetriever,
		slot:           slot,
		libLoader:      defaultLoader,
	}
}

// Public is a required method of the crypto.Signer interface
func (ys *yubikeySigner) Public() crypto.PublicKey {
	publicKey, err := x509.ParsePKIXPublicKey(ys.YubiPrivateKey.Public())
	if err != nil {
		return nil
	}

	return publicKey
}

func (y *YubiPrivateKey) setLibLoader(loader pkcs11LibLoader) {
	y.libLoader = loader
}

// CryptoSigner returns a crypto.Signer tha wraps the YubiPrivateKey. Needed for
// Certificate generation only
func (y *YubiPrivateKey) CryptoSigner() crypto.Signer {
	return &yubikeySigner{YubiPrivateKey: *y}
}

// Private is not implemented in hardware  keys
func (y *YubiPrivateKey) Private() []byte {
	// We cannot return the private material from a Yubikey
	// TODO(david): We probably want to return an error here
	return nil
}

// SignatureAlgorithm returns which algorithm this key uses to sign - currently
// hardcoded to ECDSA
func (y YubiPrivateKey) SignatureAlgorithm() data.SigAlgorithm {
	return data.ECDSASignature
}

// Sign is a required method of the crypto.Signer interface and the data.PrivateKey
// interface
func (y *YubiPrivateKey) Sign(rand io.Reader, msg []byte, opts crypto.SignerOpts) ([]byte, error) {
	ctx, session, err := SetupHSMEnv(pkcs11Lib, y.libLoader)
	if err != nil {
		return nil, err
	}
	defer cleanup(ctx, session)

	v := signed.Verifiers[data.ECDSASignature]
	for i := 0; i < sigAttempts; i++ {
		sig, err := sign(ctx, session, y.slot, y.passRetriever, msg)
		if err != nil {
			return nil, fmt.Errorf("failed to sign using Yubikey: %v", err)
		}
		if err := v.Verify(&y.ECDSAPublicKey, sig, msg); err == nil {
			return sig, nil
		}
	}
	return nil, errors.New("Failed to generate signature on Yubikey.")
}

// If a byte array is less than the number of bytes specified by
// ecdsaPrivateKeySize, left-zero-pad the byte array until
// it is the required size.
func ensurePrivateKeySize(payload []byte) []byte {
	final := payload
	if len(payload) < ecdsaPrivateKeySize {
		final = make([]byte, ecdsaPrivateKeySize)
		copy(final[ecdsaPrivateKeySize-len(payload):], payload)
	}
	return final
}

// addECDSAKey adds a key to the yubikey
func addECDSAKey(
	ctx IPKCS11Ctx,
	session pkcs11.SessionHandle,
	privKey data.PrivateKey,
	pkcs11KeyID []byte,
	passRetriever passphrase.Retriever,
	role string,
) error {
	logrus.Debugf("Attempting to add key to yubikey with ID: %s", privKey.ID())

	err := login(ctx, session, passRetriever, pkcs11.CKU_SO, SOUserPin)
	if err != nil {
		return err
	}
	defer ctx.Logout(session)

	// Create an ecdsa.PrivateKey out of the private key bytes
	ecdsaPrivKey, err := x509.ParseECPrivateKey(privKey.Private())
	if err != nil {
		return err
	}

	ecdsaPrivKeyD := ensurePrivateKeySize(ecdsaPrivKey.D.Bytes())

	// Hard-coded policy: the generated certificate expires in 10 years.
	startTime := time.Now()
	template, err := trustmanager.NewCertificate(role, startTime, startTime.AddDate(10, 0, 0))
	if err != nil {
		return fmt.Errorf("failed to create the certificate template: %v", err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, ecdsaPrivKey.Public(), ecdsaPrivKey)
	if err != nil {
		return fmt.Errorf("failed to create the certificate: %v", err)
	}

	certTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_CERTIFICATE),
		pkcs11.NewAttribute(pkcs11.CKA_VALUE, certBytes),
		pkcs11.NewAttribute(pkcs11.CKA_ID, pkcs11KeyID),
	}

	privateKeyTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_ECDSA),
		pkcs11.NewAttribute(pkcs11.CKA_ID, pkcs11KeyID),
		pkcs11.NewAttribute(pkcs11.CKA_EC_PARAMS, []byte{0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x03, 0x01, 0x07}),
		pkcs11.NewAttribute(pkcs11.CKA_VALUE, ecdsaPrivKeyD),
		pkcs11.NewAttribute(pkcs11.CKA_VENDOR_DEFINED, yubikeyKeymode),
	}

	_, err = ctx.CreateObject(session, certTemplate)
	if err != nil {
		return fmt.Errorf("error importing: %v", err)
	}

	_, err = ctx.CreateObject(session, privateKeyTemplate)
	if err != nil {
		return fmt.Errorf("error importing: %v", err)
	}

	return nil
}

func getECDSAKey(ctx IPKCS11Ctx, session pkcs11.SessionHandle, pkcs11KeyID []byte) (*data.ECDSAPublicKey, string, error) {
	findTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, true),
		pkcs11.NewAttribute(pkcs11.CKA_ID, pkcs11KeyID),
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PUBLIC_KEY),
	}

	attrTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, []byte{0}),
		pkcs11.NewAttribute(pkcs11.CKA_EC_POINT, []byte{0}),
		pkcs11.NewAttribute(pkcs11.CKA_EC_PARAMS, []byte{0}),
	}

	if err := ctx.FindObjectsInit(session, findTemplate); err != nil {
		logrus.Debugf("Failed to init: %s", err.Error())
		return nil, "", err
	}
	obj, _, err := ctx.FindObjects(session, 1)
	if err != nil {
		logrus.Debugf("Failed to find objects: %v", err)
		return nil, "", err
	}
	if err := ctx.FindObjectsFinal(session); err != nil {
		logrus.Debugf("Failed to finalize: %s", err.Error())
		return nil, "", err
	}
	if len(obj) != 1 {
		logrus.Debugf("should have found one object")
		return nil, "", errors.New("no matching keys found inside of yubikey")
	}

	// Retrieve the public-key material to be able to create a new ECSAKey
	attr, err := ctx.GetAttributeValue(session, obj[0], attrTemplate)
	if err != nil {
		logrus.Debugf("Failed to get Attribute for: %v", obj[0])
		return nil, "", err
	}

	// Iterate through all the attributes of this key and saves CKA_PUBLIC_EXPONENT and CKA_MODULUS. Removes ordering specific issues.
	var rawPubKey []byte
	for _, a := range attr {
		if a.Type == pkcs11.CKA_EC_POINT {
			rawPubKey = a.Value
		}

	}

	ecdsaPubKey := ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(rawPubKey[3:35]), Y: new(big.Int).SetBytes(rawPubKey[35:])}
	pubBytes, err := x509.MarshalPKIXPublicKey(&ecdsaPubKey)
	if err != nil {
		logrus.Debugf("Failed to Marshal public key")
		return nil, "", err
	}

	return data.NewECDSAPublicKey(pubBytes), data.CanonicalRootRole, nil
}

// sign returns a signature for a given signature request
func sign(ctx IPKCS11Ctx, session pkcs11.SessionHandle, pkcs11KeyID []byte, passRetriever passphrase.Retriever, payload []byte) ([]byte, error) {
	err := login(ctx, session, passRetriever, pkcs11.CKU_USER, UserPin)
	if err != nil {
		return nil, fmt.Errorf("error logging in: %v", err)
	}
	defer ctx.Logout(session)

	// Define the ECDSA Private key template
	class := pkcs11.CKO_PRIVATE_KEY
	privateKeyTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, class),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_ECDSA),
		pkcs11.NewAttribute(pkcs11.CKA_ID, pkcs11KeyID),
	}

	if err := ctx.FindObjectsInit(session, privateKeyTemplate); err != nil {
		logrus.Debugf("Failed to init find objects: %s", err.Error())
		return nil, err
	}
	obj, _, err := ctx.FindObjects(session, 1)
	if err != nil {
		logrus.Debugf("Failed to find objects: %v", err)
		return nil, err
	}
	if err = ctx.FindObjectsFinal(session); err != nil {
		logrus.Debugf("Failed to finalize find objects: %s", err.Error())
		return nil, err
	}
	if len(obj) != 1 {
		return nil, errors.New("length of objects found not 1")
	}

	var sig []byte
	err = ctx.SignInit(
		session, []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_ECDSA, nil)}, obj[0])
	if err != nil {
		return nil, err
	}

	// Get the SHA256 of the payload
	digest := sha256.Sum256(payload)

	if (yubikeyKeymode & KeymodeTouch) > 0 {
		touchToSignUI()
		defer touchDoneCallback()
	}
	// a call to Sign, whether or not Sign fails, will clear the SignInit
	sig, err = ctx.Sign(session, digest[:])
	if err != nil {
		logrus.Debugf("Error while signing: %s", err)
		return nil, err
	}

	if sig == nil {
		return nil, errors.New("Failed to create signature")
	}
	return sig[:], nil
}

func yubiRemoveKey(ctx IPKCS11Ctx, session pkcs11.SessionHandle, pkcs11KeyID []byte, passRetriever passphrase.Retriever, keyID string) error {
	err := login(ctx, session, passRetriever, pkcs11.CKU_SO, SOUserPin)
	if err != nil {
		return err
	}
	defer ctx.Logout(session)

	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, true),
		pkcs11.NewAttribute(pkcs11.CKA_ID, pkcs11KeyID),
		//pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_CERTIFICATE),
	}

	if err := ctx.FindObjectsInit(session, template); err != nil {
		logrus.Debugf("Failed to init find objects: %s", err.Error())
		return err
	}
	obj, b, err := ctx.FindObjects(session, 1)
	if err != nil {
		logrus.Debugf("Failed to find objects: %s %v", err.Error(), b)
		return err
	}
	if err := ctx.FindObjectsFinal(session); err != nil {
		logrus.Debugf("Failed to finalize find objects: %s", err.Error())
		return err
	}
	if len(obj) != 1 {
		logrus.Debugf("should have found exactly one object")
		return err
	}

	// Delete the certificate
	err = ctx.DestroyObject(session, obj[0])
	if err != nil {
		logrus.Debugf("Failed to delete cert")
		return err
	}
	return nil
}

func yubiListKeys(ctx IPKCS11Ctx, session pkcs11.SessionHandle) (keys map[string]yubiSlot, err error) {
	keys = make(map[string]yubiSlot)
	findTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, true),
		//pkcs11.NewAttribute(pkcs11.CKA_ID, pkcs11KeyID),
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_CERTIFICATE),
	}

	attrTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_ID, []byte{0}),
		pkcs11.NewAttribute(pkcs11.CKA_VALUE, []byte{0}),
	}

	if err = ctx.FindObjectsInit(session, findTemplate); err != nil {
		logrus.Debugf("Failed to init: %s", err.Error())
		return
	}
	objs, b, err := ctx.FindObjects(session, numSlots)
	for err == nil {
		var o []pkcs11.ObjectHandle
		o, b, err = ctx.FindObjects(session, numSlots)
		if err != nil {
			continue
		}
		if len(o) == 0 {
			break
		}
		objs = append(objs, o...)
	}
	if err != nil {
		logrus.Debugf("Failed to find: %s %v", err.Error(), b)
		if len(objs) == 0 {
			return nil, err
		}
	}
	if err = ctx.FindObjectsFinal(session); err != nil {
		logrus.Debugf("Failed to finalize: %s", err.Error())
		return
	}
	if len(objs) == 0 {
		return nil, errors.New("No keys found in yubikey.")
	}
	logrus.Debugf("Found %d objects matching list filters", len(objs))
	for _, obj := range objs {
		var (
			cert *x509.Certificate
			slot []byte
		)
		// Retrieve the public-key material to be able to create a new ECDSA
		attr, err := ctx.GetAttributeValue(session, obj, attrTemplate)
		if err != nil {
			logrus.Debugf("Failed to get Attribute for: %v", obj)
			continue
		}

		// Iterate through all the attributes of this key and saves CKA_PUBLIC_EXPONENT and CKA_MODULUS. Removes ordering specific issues.
		for _, a := range attr {
			if a.Type == pkcs11.CKA_ID {
				slot = a.Value
			}
			if a.Type == pkcs11.CKA_VALUE {
				cert, err = x509.ParseCertificate(a.Value)
				if err != nil {
					continue
				}
				if !data.ValidRole(cert.Subject.CommonName) {
					continue
				}
			}
		}

		// we found nothing
		if cert == nil {
			continue
		}

		var ecdsaPubKey *ecdsa.PublicKey
		switch cert.PublicKeyAlgorithm {
		case x509.ECDSA:
			ecdsaPubKey = cert.PublicKey.(*ecdsa.PublicKey)
		default:
			logrus.Infof("Unsupported x509 PublicKeyAlgorithm: %d", cert.PublicKeyAlgorithm)
			continue
		}

		pubBytes, err := x509.MarshalPKIXPublicKey(ecdsaPubKey)
		if err != nil {
			logrus.Debugf("Failed to Marshal public key")
			continue
		}

		keys[data.NewECDSAPublicKey(pubBytes).ID()] = yubiSlot{
			role:   cert.Subject.CommonName,
			slotID: slot,
		}
	}
	return
}

func getNextEmptySlot(ctx IPKCS11Ctx, session pkcs11.SessionHandle) ([]byte, error) {
	findTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, true),
	}
	attrTemplate := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_ID, []byte{0}),
	}

	if err := ctx.FindObjectsInit(session, findTemplate); err != nil {
		logrus.Debugf("Failed to init: %s", err.Error())
		return nil, err
	}
	objs, b, err := ctx.FindObjects(session, numSlots)
	// if there are more objects than `numSlots`, get all of them until
	// there are no more to get
	for err == nil {
		var o []pkcs11.ObjectHandle
		o, b, err = ctx.FindObjects(session, numSlots)
		if err != nil {
			continue
		}
		if len(o) == 0 {
			break
		}
		objs = append(objs, o...)
	}
	taken := make(map[int]bool)
	if err != nil {
		logrus.Debugf("Failed to find: %s %v", err.Error(), b)
		return nil, err
	}
	if err = ctx.FindObjectsFinal(session); err != nil {
		logrus.Debugf("Failed to finalize: %s\n", err.Error())
		return nil, err
	}
	for _, obj := range objs {
		// Retrieve the slot ID
		attr, err := ctx.GetAttributeValue(session, obj, attrTemplate)
		if err != nil {
			continue
		}

		// Iterate through attributes. If an ID attr was found, mark it as taken
		for _, a := range attr {
			if a.Type == pkcs11.CKA_ID {
				if len(a.Value) < 1 {
					continue
				}
				// a byte will always be capable of representing all slot IDs
				// for the Yubikeys
				slotNum := int(a.Value[0])
				if slotNum >= numSlots {
					// defensive
					continue
				}
				taken[slotNum] = true
			}
		}
	}
	// iterate the token locations in our preferred order and use the first
	// available one. Otherwise exit the loop and return an error.
	for _, loc := range slotIDs {
		if !taken[loc] {
			return []byte{byte(loc)}, nil
		}
	}
	return nil, errors.New("Yubikey has no available slots.")
}

// YubiStore is a KeyStore for private keys inside a Yubikey
type YubiStore struct {
	passRetriever passphrase.Retriever
	keys          map[string]yubiSlot
	backupStore   trustmanager.KeyStore
	libLoader     pkcs11LibLoader
}

// NewYubiStore returns a YubiStore, given a backup key store to write any
// generated keys to (usually a KeyFileStore)
func NewYubiStore(backupStore trustmanager.KeyStore, passphraseRetriever passphrase.Retriever) (
	*YubiStore, error) {

	s := &YubiStore{
		passRetriever: passphraseRetriever,
		keys:          make(map[string]yubiSlot),
		backupStore:   backupStore,
		libLoader:     defaultLoader,
	}
	s.ListKeys() // populate keys field
	return s, nil
}

// Name returns a user friendly name for the location this store
// keeps its data
func (s YubiStore) Name() string {
	return "yubikey"
}

func (s *YubiStore) setLibLoader(loader pkcs11LibLoader) {
	s.libLoader = loader
}

// ListKeys returns a list of keys in the yubikey store
func (s *YubiStore) ListKeys() map[string]trustmanager.KeyInfo {
	if len(s.keys) > 0 {
		return buildKeyMap(s.keys)
	}
	ctx, session, err := SetupHSMEnv(pkcs11Lib, s.libLoader)
	if err != nil {
		logrus.Debugf("Failed to initialize PKCS11 environment: %s", err.Error())
		return nil
	}
	defer cleanup(ctx, session)

	keys, err := yubiListKeys(ctx, session)
	if err != nil {
		logrus.Debugf("Failed to list key from the yubikey: %s", err.Error())
		return nil
	}
	s.keys = keys

	return buildKeyMap(keys)
}

// AddKey puts a key inside the Yubikey, as well as writing it to the backup store
func (s *YubiStore) AddKey(keyInfo trustmanager.KeyInfo, privKey data.PrivateKey) error {
	added, err := s.addKey(privKey.ID(), keyInfo.Role, privKey)
	if err != nil {
		return err
	}
	if added && s.backupStore != nil {
		err = s.backupStore.AddKey(keyInfo, privKey)
		if err != nil {
			defer s.RemoveKey(privKey.ID())
			return ErrBackupFailed{err: err.Error()}
		}
	}
	return nil
}

// Only add if we haven't seen the key already.  Return whether the key was
// added.
func (s *YubiStore) addKey(keyID, role string, privKey data.PrivateKey) (
	bool, error) {

	// We only allow adding root keys for now
	if role != data.CanonicalRootRole {
		return false, fmt.Errorf(
			"yubikey only supports storing root keys, got %s for key: %s", role, keyID)
	}

	ctx, session, err := SetupHSMEnv(pkcs11Lib, s.libLoader)
	if err != nil {
		logrus.Debugf("Failed to initialize PKCS11 environment: %s", err.Error())
		return false, err
	}
	defer cleanup(ctx, session)

	if k, ok := s.keys[keyID]; ok {
		if k.role == role {
			// already have the key and it's associated with the correct role
			return false, nil
		}
	}

	slot, err := getNextEmptySlot(ctx, session)
	if err != nil {
		logrus.Debugf("Failed to get an empty yubikey slot: %s", err.Error())
		return false, err
	}
	logrus.Debugf("Attempting to store key using yubikey slot %v", slot)

	err = addECDSAKey(
		ctx, session, privKey, slot, s.passRetriever, role)
	if err == nil {
		s.keys[privKey.ID()] = yubiSlot{
			role:   role,
			slotID: slot,
		}
		return true, nil
	}
	logrus.Debugf("Failed to add key to yubikey: %v", err)

	return false, err
}

// GetKey retrieves a key from the Yubikey only (it does not look inside the
// backup store)
func (s *YubiStore) GetKey(keyID string) (data.PrivateKey, string, error) {
	ctx, session, err := SetupHSMEnv(pkcs11Lib, s.libLoader)
	if err != nil {
		logrus.Debugf("Failed to initialize PKCS11 environment: %s", err.Error())
		if _, ok := err.(errHSMNotPresent); ok {
			err = trustmanager.ErrKeyNotFound{KeyID: keyID}
		}
		return nil, "", err
	}
	defer cleanup(ctx, session)

	key, ok := s.keys[keyID]
	if !ok {
		return nil, "", trustmanager.ErrKeyNotFound{KeyID: keyID}
	}

	pubKey, alias, err := getECDSAKey(ctx, session, key.slotID)
	if err != nil {
		logrus.Debugf("Failed to get key from slot %s: %s", key.slotID, err.Error())
		return nil, "", err
	}
	// Check to see if we're returning the intended keyID
	if pubKey.ID() != keyID {
		return nil, "", fmt.Errorf("expected root key: %s, but found: %s", keyID, pubKey.ID())
	}
	privKey := NewYubiPrivateKey(key.slotID, *pubKey, s.passRetriever)
	if privKey == nil {
		return nil, "", errors.New("could not initialize new YubiPrivateKey")
	}

	return privKey, alias, err
}

// RemoveKey deletes a key from the Yubikey only (it does not remove it from the
// backup store)
func (s *YubiStore) RemoveKey(keyID string) error {
	ctx, session, err := SetupHSMEnv(pkcs11Lib, s.libLoader)
	if err != nil {
		logrus.Debugf("Failed to initialize PKCS11 environment: %s", err.Error())
		return nil
	}
	defer cleanup(ctx, session)

	key, ok := s.keys[keyID]
	if !ok {
		return errors.New("Key not present in yubikey")
	}
	err = yubiRemoveKey(ctx, session, key.slotID, s.passRetriever, keyID)
	if err == nil {
		delete(s.keys, keyID)
	} else {
		logrus.Debugf("Failed to remove from the yubikey KeyID %s: %v", keyID, err)
	}

	return err
}

// ExportKey doesn't work, because you can't export data from a Yubikey
func (s *YubiStore) ExportKey(keyID string) ([]byte, error) {
	logrus.Debugf("Attempting to export: %s key inside of YubiStore", keyID)
	return nil, errors.New("Keys cannot be exported from a Yubikey.")
}

// GetKeyInfo is not yet implemented
func (s *YubiStore) GetKeyInfo(keyID string) (trustmanager.KeyInfo, error) {
	return trustmanager.KeyInfo{}, fmt.Errorf("Not yet implemented")
}

func cleanup(ctx IPKCS11Ctx, session pkcs11.SessionHandle) {
	err := ctx.CloseSession(session)
	if err != nil {
		logrus.Debugf("Error closing session: %s", err.Error())
	}
	finalizeAndDestroy(ctx)
}

func finalizeAndDestroy(ctx IPKCS11Ctx) {
	err := ctx.Finalize()
	if err != nil {
		logrus.Debugf("Error finalizing: %s", err.Error())
	}
	ctx.Destroy()
}

// SetupHSMEnv is a method that depends on the existences
func SetupHSMEnv(libraryPath string, libLoader pkcs11LibLoader) (
	IPKCS11Ctx, pkcs11.SessionHandle, error) {

	if libraryPath == "" {
		return nil, 0, errHSMNotPresent{err: "no library found"}
	}
	p := libLoader(libraryPath)

	if p == nil {
		return nil, 0, fmt.Errorf("failed to load library %s", libraryPath)
	}

	if err := p.Initialize(); err != nil {
		defer finalizeAndDestroy(p)
		return nil, 0, fmt.Errorf("found library %s, but initialize error %s", libraryPath, err.Error())
	}

	slots, err := p.GetSlotList(true)
	if err != nil {
		defer finalizeAndDestroy(p)
		return nil, 0, fmt.Errorf(
			"loaded library %s, but failed to list HSM slots %s", libraryPath, err)
	}
	// Check to see if we got any slots from the HSM.
	if len(slots) < 1 {
		defer finalizeAndDestroy(p)
		return nil, 0, fmt.Errorf(
			"loaded library %s, but no HSM slots found", libraryPath)
	}

	// CKF_SERIAL_SESSION: TRUE if cryptographic functions are performed in serial with the application; FALSE if the functions may be performed in parallel with the application.
	// CKF_RW_SESSION: TRUE if the session is read/write; FALSE if the session is read-only
	session, err := p.OpenSession(slots[0], pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		defer cleanup(p, session)
		return nil, 0, fmt.Errorf(
			"loaded library %s, but failed to start session with HSM %s",
			libraryPath, err)
	}

	logrus.Debugf("Initialized PKCS11 library %s and started HSM session", libraryPath)
	return p, session, nil
}

// IsAccessible returns true if a Yubikey can be accessed
func IsAccessible() bool {
	if pkcs11Lib == "" {
		return false
	}
	ctx, session, err := SetupHSMEnv(pkcs11Lib, defaultLoader)
	if err != nil {
		return false
	}
	defer cleanup(ctx, session)
	return true
}

func login(ctx IPKCS11Ctx, session pkcs11.SessionHandle, passRetriever passphrase.Retriever, userFlag uint, defaultPassw string) error {
	// try default password
	err := ctx.Login(session, userFlag, defaultPassw)
	if err == nil {
		return nil
	}

	// default failed, ask user for password
	for attempts := 0; ; attempts++ {
		var (
			giveup bool
			err    error
			user   string
		)
		if userFlag == pkcs11.CKU_SO {
			user = "SO Pin"
		} else {
			user = "User Pin"
		}
		passwd, giveup, err := passRetriever(user, "yubikey", false, attempts)
		// Check if the passphrase retriever got an error or if it is telling us to give up
		if giveup || err != nil {
			return trustmanager.ErrPasswordInvalid{}
		}
		if attempts > 2 {
			return trustmanager.ErrAttemptsExceeded{}
		}

		// Try to convert PEM encoded bytes back to a PrivateKey using the passphrase
		err = ctx.Login(session, userFlag, passwd)
		if err == nil {
			return nil
		}
	}
	return nil
}

func buildKeyMap(keys map[string]yubiSlot) map[string]trustmanager.KeyInfo {
	res := make(map[string]trustmanager.KeyInfo)
	for k, v := range keys {
		res[k] = trustmanager.KeyInfo{Role: v.role, Gun: ""}
	}
	return res
}
