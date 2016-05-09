package trustmanager

import (
	"encoding/pem"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/notary"
	"github.com/docker/notary/passphrase"
	"github.com/docker/notary/tuf/data"
)

type keyInfoMap map[string]KeyInfo

// KeyFileStore persists and manages private keys on disk
type KeyFileStore struct {
	sync.Mutex
	SimpleFileStore
	passphrase.Retriever
	cachedKeys map[string]*cachedKey
	keyInfoMap
}

// KeyMemoryStore manages private keys in memory
type KeyMemoryStore struct {
	sync.Mutex
	MemoryFileStore
	passphrase.Retriever
	cachedKeys map[string]*cachedKey
	keyInfoMap
}

// KeyInfo stores the role, path, and gun for a corresponding private key ID
// It is assumed that each private key ID is unique
type KeyInfo struct {
	Gun  string
	Role string
}

// NewKeyFileStore returns a new KeyFileStore creating a private directory to
// hold the keys.
func NewKeyFileStore(baseDir string, passphraseRetriever passphrase.Retriever) (*KeyFileStore, error) {
	baseDir = filepath.Join(baseDir, notary.PrivDir)
	fileStore, err := NewPrivateSimpleFileStore(baseDir, keyExtension)
	if err != nil {
		return nil, err
	}
	cachedKeys := make(map[string]*cachedKey)
	keyInfoMap := make(keyInfoMap)

	keyStore := &KeyFileStore{SimpleFileStore: *fileStore,
		Retriever:  passphraseRetriever,
		cachedKeys: cachedKeys,
		keyInfoMap: keyInfoMap,
	}

	// Load this keystore's ID --> gun/role map
	keyStore.loadKeyInfo()
	return keyStore, nil
}

func generateKeyInfoMap(s Storage) map[string]KeyInfo {
	keyInfoMap := make(map[string]KeyInfo)
	for _, keyPath := range s.ListFiles() {
		d, err := s.Get(keyPath)
		if err != nil {
			logrus.Error(err)
			continue
		}
		keyID, keyInfo, err := KeyInfoFromPEM(d, keyPath)
		if err != nil {
			logrus.Error(err)
			continue
		}
		keyInfoMap[keyID] = keyInfo
	}
	return keyInfoMap
}

// Attempts to infer the keyID, role, and GUN from the specified key path.
// Note that non-root roles can only be inferred if this is a legacy style filename: KEYID_ROLE.key
func inferKeyInfoFromKeyPath(keyPath string) (string, string, string) {
	var keyID, role, gun string
	keyID = filepath.Base(keyPath)
	underscoreIndex := strings.LastIndex(keyID, "_")

	// This is the legacy KEYID_ROLE filename
	// The keyID is the first part of the keyname
	// The keyRole is the second part of the keyname
	// in a key named abcde_root, abcde is the keyID and root is the KeyAlias
	if underscoreIndex != -1 {
		role = keyID[underscoreIndex+1:]
		keyID = keyID[:underscoreIndex]
	}

	if filepath.HasPrefix(keyPath, notary.RootKeysSubdir+"/") {
		return keyID, data.CanonicalRootRole, ""
	}

	keyPath = strings.TrimPrefix(keyPath, notary.NonRootKeysSubdir+"/")
	gun = getGunFromFullID(keyPath)
	return keyID, role, gun
}

func getGunFromFullID(fullKeyID string) string {
	keyGun := filepath.Dir(fullKeyID)
	// If the gun is empty, Dir will return .
	if keyGun == "." {
		keyGun = ""
	}
	return keyGun
}

func (s *KeyFileStore) loadKeyInfo() {
	s.keyInfoMap = generateKeyInfoMap(s)
}

func (s *KeyMemoryStore) loadKeyInfo() {
	s.keyInfoMap = generateKeyInfoMap(s)
}

// GetKeyInfo returns the corresponding gun and role key info for a keyID
func (s *KeyFileStore) GetKeyInfo(keyID string) (KeyInfo, error) {
	if info, ok := s.keyInfoMap[keyID]; ok {
		return info, nil
	}
	return KeyInfo{}, fmt.Errorf("Could not find info for keyID %s", keyID)
}

// GetKeyInfo returns the corresponding gun and role key info for a keyID
func (s *KeyMemoryStore) GetKeyInfo(keyID string) (KeyInfo, error) {
	if info, ok := s.keyInfoMap[keyID]; ok {
		return info, nil
	}
	return KeyInfo{}, fmt.Errorf("Could not find info for keyID %s", keyID)
}

// Name returns a user friendly name for the location this store
// keeps its data
func (s *KeyFileStore) Name() string {
	return fmt.Sprintf("file (%s)", s.SimpleFileStore.BaseDir())
}

// AddKey stores the contents of a PEM-encoded private key as a PEM block
func (s *KeyFileStore) AddKey(keyInfo KeyInfo, privKey data.PrivateKey) error {
	s.Lock()
	defer s.Unlock()
	if keyInfo.Role == data.CanonicalRootRole || data.IsDelegation(keyInfo.Role) || !data.ValidRole(keyInfo.Role) {
		keyInfo.Gun = ""
	}
	err := addKey(s, s.Retriever, s.cachedKeys, filepath.Join(keyInfo.Gun, privKey.ID()), keyInfo.Role, privKey)
	if err != nil {
		return err
	}
	s.keyInfoMap[privKey.ID()] = keyInfo
	return nil
}

// GetKey returns the PrivateKey given a KeyID
func (s *KeyFileStore) GetKey(name string) (data.PrivateKey, string, error) {
	s.Lock()
	defer s.Unlock()
	// If this is a bare key ID without the gun, prepend the gun so the filestore lookup succeeds
	if keyInfo, ok := s.keyInfoMap[name]; ok {
		name = filepath.Join(keyInfo.Gun, name)
	}
	return getKey(s, s.Retriever, s.cachedKeys, name)
}

// ListKeys returns a list of unique PublicKeys present on the KeyFileStore, by returning a copy of the keyInfoMap
func (s *KeyFileStore) ListKeys() map[string]KeyInfo {
	return copyKeyInfoMap(s.keyInfoMap)
}

// RemoveKey removes the key from the keyfilestore
func (s *KeyFileStore) RemoveKey(keyID string) error {
	s.Lock()
	defer s.Unlock()
	// If this is a bare key ID without the gun, prepend the gun so the filestore lookup succeeds
	if keyInfo, ok := s.keyInfoMap[keyID]; ok {
		keyID = filepath.Join(keyInfo.Gun, keyID)
	}
	err := removeKey(s, s.cachedKeys, keyID)
	if err != nil {
		return err
	}
	// Remove this key from our keyInfo map if we removed from our filesystem
	delete(s.keyInfoMap, filepath.Base(keyID))
	return nil
}

// ExportKey exports the encrypted bytes from the keystore
func (s *KeyFileStore) ExportKey(keyID string) ([]byte, error) {
	if keyInfo, ok := s.keyInfoMap[keyID]; ok {
		keyID = filepath.Join(keyInfo.Gun, keyID)
	}
	keyBytes, _, err := getRawKey(s, keyID)
	if err != nil {
		return nil, err
	}
	return keyBytes, nil
}

// NewKeyMemoryStore returns a new KeyMemoryStore which holds keys in memory
func NewKeyMemoryStore(passphraseRetriever passphrase.Retriever) *KeyMemoryStore {
	memStore := NewMemoryFileStore()
	cachedKeys := make(map[string]*cachedKey)

	keyInfoMap := make(keyInfoMap)

	keyStore := &KeyMemoryStore{MemoryFileStore: *memStore,
		Retriever:  passphraseRetriever,
		cachedKeys: cachedKeys,
		keyInfoMap: keyInfoMap,
	}

	// Load this keystore's ID --> gun/role map
	keyStore.loadKeyInfo()
	return keyStore
}

// Name returns a user friendly name for the location this store
// keeps its data
func (s *KeyMemoryStore) Name() string {
	return "memory"
}

// AddKey stores the contents of a PEM-encoded private key as a PEM block
func (s *KeyMemoryStore) AddKey(keyInfo KeyInfo, privKey data.PrivateKey) error {
	s.Lock()
	defer s.Unlock()
	if keyInfo.Role == data.CanonicalRootRole || data.IsDelegation(keyInfo.Role) || !data.ValidRole(keyInfo.Role) {
		keyInfo.Gun = ""
	}
	err := addKey(s, s.Retriever, s.cachedKeys, filepath.Join(keyInfo.Gun, privKey.ID()), keyInfo.Role, privKey)
	if err != nil {
		return err
	}
	s.keyInfoMap[privKey.ID()] = keyInfo
	return nil
}

// GetKey returns the PrivateKey given a KeyID
func (s *KeyMemoryStore) GetKey(name string) (data.PrivateKey, string, error) {
	s.Lock()
	defer s.Unlock()
	// If this is a bare key ID without the gun, prepend the gun so the filestore lookup succeeds
	if keyInfo, ok := s.keyInfoMap[name]; ok {
		name = filepath.Join(keyInfo.Gun, name)
	}
	return getKey(s, s.Retriever, s.cachedKeys, name)
}

// ListKeys returns a list of unique PublicKeys present on the KeyFileStore, by returning a copy of the keyInfoMap
func (s *KeyMemoryStore) ListKeys() map[string]KeyInfo {
	return copyKeyInfoMap(s.keyInfoMap)
}

// copyKeyInfoMap returns a deep copy of the passed-in keyInfoMap
func copyKeyInfoMap(keyInfoMap map[string]KeyInfo) map[string]KeyInfo {
	copyMap := make(map[string]KeyInfo)
	for keyID, keyInfo := range keyInfoMap {
		copyMap[keyID] = KeyInfo{Role: keyInfo.Role, Gun: keyInfo.Gun}
	}
	return copyMap
}

// RemoveKey removes the key from the keystore
func (s *KeyMemoryStore) RemoveKey(keyID string) error {
	s.Lock()
	defer s.Unlock()
	// If this is a bare key ID without the gun, prepend the gun so the filestore lookup succeeds
	if keyInfo, ok := s.keyInfoMap[keyID]; ok {
		keyID = filepath.Join(keyInfo.Gun, keyID)
	}
	err := removeKey(s, s.cachedKeys, keyID)
	if err != nil {
		return err
	}
	// Remove this key from our keyInfo map if we removed from our filesystem
	delete(s.keyInfoMap, filepath.Base(keyID))
	return nil
}

// ExportKey exports the encrypted bytes from the keystore
func (s *KeyMemoryStore) ExportKey(keyID string) ([]byte, error) {
	keyBytes, _, err := getRawKey(s, keyID)
	if err != nil {
		return nil, err
	}
	return keyBytes, nil
}

// KeyInfoFromPEM attempts to get a keyID and KeyInfo from the filename and PEM bytes of a key
func KeyInfoFromPEM(pemBytes []byte, filename string) (string, KeyInfo, error) {
	keyID, role, gun := inferKeyInfoFromKeyPath(filename)
	if role == "" {
		block, _ := pem.Decode(pemBytes)
		if block == nil {
			return "", KeyInfo{}, fmt.Errorf("could not decode PEM block for key %s", filename)
		}
		if keyRole, ok := block.Headers["role"]; ok {
			role = keyRole
		}
	}
	return keyID, KeyInfo{Gun: gun, Role: role}, nil
}

func addKey(s Storage, passphraseRetriever passphrase.Retriever, cachedKeys map[string]*cachedKey, name, role string, privKey data.PrivateKey) error {

	var (
		chosenPassphrase string
		giveup           bool
		err              error
	)

	for attempts := 0; ; attempts++ {
		chosenPassphrase, giveup, err = passphraseRetriever(name, role, true, attempts)
		if err != nil {
			continue
		}
		if giveup {
			return ErrAttemptsExceeded{}
		}
		if attempts > 10 {
			return ErrAttemptsExceeded{}
		}
		break
	}

	return encryptAndAddKey(s, chosenPassphrase, cachedKeys, name, role, privKey)
}

// getKeyRole finds the role for the given keyID. It attempts to look
// both in the newer format PEM headers, and also in the legacy filename
// format. It returns: the role, whether it was found in the legacy format
// (true == legacy), and an error
func getKeyRole(s Storage, keyID string) (string, bool, error) {
	name := strings.TrimSpace(strings.TrimSuffix(filepath.Base(keyID), filepath.Ext(keyID)))

	for _, file := range s.ListFiles() {
		filename := filepath.Base(file)

		if strings.HasPrefix(filename, name) {
			d, err := s.Get(file)
			if err != nil {
				return "", false, err
			}
			block, _ := pem.Decode(d)
			if block != nil {
				if role, ok := block.Headers["role"]; ok {
					return role, false, nil
				}
			}

			role := strings.TrimPrefix(filename, name+"_")
			return role, true, nil
		}
	}

	return "", false, ErrKeyNotFound{KeyID: keyID}
}

// GetKey returns the PrivateKey given a KeyID
func getKey(s Storage, passphraseRetriever passphrase.Retriever, cachedKeys map[string]*cachedKey, name string) (data.PrivateKey, string, error) {
	cachedKeyEntry, ok := cachedKeys[name]
	if ok {
		return cachedKeyEntry.key, cachedKeyEntry.alias, nil
	}

	keyBytes, keyAlias, err := getRawKey(s, name)
	if err != nil {
		return nil, "", err
	}

	// See if the key is encrypted. If its encrypted we'll fail to parse the private key
	privKey, err := ParsePEMPrivateKey(keyBytes, "")
	if err != nil {
		privKey, _, err = GetPasswdDecryptBytes(passphraseRetriever, keyBytes, name, string(keyAlias))
		if err != nil {
			return nil, "", err
		}
	}
	cachedKeys[name] = &cachedKey{alias: keyAlias, key: privKey}
	return privKey, keyAlias, nil
}

// RemoveKey removes the key from the keyfilestore
func removeKey(s Storage, cachedKeys map[string]*cachedKey, name string) error {
	role, legacy, err := getKeyRole(s, name)
	if err != nil {
		return err
	}

	delete(cachedKeys, name)

	if legacy {
		name = name + "_" + role
	}

	// being in a subdirectory is for backwards compatibliity
	err = s.Remove(filepath.Join(getSubdir(role), name))
	if err != nil {
		return err
	}
	return nil
}

// Assumes 2 subdirectories, 1 containing root keys and 1 containing tuf keys
func getSubdir(alias string) string {
	if alias == data.CanonicalRootRole {
		return notary.RootKeysSubdir
	}
	return notary.NonRootKeysSubdir
}

// Given a key ID, gets the bytes and alias belonging to that key if the key
// exists
func getRawKey(s Storage, name string) ([]byte, string, error) {
	role, legacy, err := getKeyRole(s, name)
	if err != nil {
		return nil, "", err
	}

	if legacy {
		name = name + "_" + role
	}

	var keyBytes []byte
	keyBytes, err = s.Get(filepath.Join(getSubdir(role), name))
	if err != nil {
		return nil, "", err
	}
	return keyBytes, role, nil
}

// GetPasswdDecryptBytes gets the password to decrypt the given pem bytes.
// Returns the password and private key
func GetPasswdDecryptBytes(passphraseRetriever passphrase.Retriever, pemBytes []byte, name, alias string) (data.PrivateKey, string, error) {
	var (
		passwd  string
		retErr  error
		privKey data.PrivateKey
	)
	for attempts := 0; ; attempts++ {
		var (
			giveup bool
			err    error
		)
		passwd, giveup, err = passphraseRetriever(name, alias, false, attempts)
		// Check if the passphrase retriever got an error or if it is telling us to give up
		if giveup || err != nil {
			return nil, "", ErrPasswordInvalid{}
		}
		if attempts > 10 {
			return nil, "", ErrAttemptsExceeded{}
		}

		// Try to convert PEM encoded bytes back to a PrivateKey using the passphrase
		privKey, err = ParsePEMPrivateKey(pemBytes, passwd)
		if err != nil {
			retErr = ErrPasswordInvalid{}
		} else {
			// We managed to parse the PrivateKey. We've succeeded!
			retErr = nil
			break
		}
	}
	if retErr != nil {
		return nil, "", retErr
	}
	return privKey, passwd, nil
}

func encryptAndAddKey(s Storage, passwd string, cachedKeys map[string]*cachedKey, name, role string, privKey data.PrivateKey) error {

	var (
		pemPrivKey []byte
		err        error
	)

	if passwd != "" {
		pemPrivKey, err = EncryptPrivateKey(privKey, role, passwd)
	} else {
		pemPrivKey, err = KeyToPEM(privKey, role)
	}

	if err != nil {
		return err
	}

	cachedKeys[name] = &cachedKey{alias: role, key: privKey}
	return s.Add(filepath.Join(getSubdir(role), name), pemPrivKey)
}
