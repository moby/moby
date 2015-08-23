package trustmanager

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/notary/pkg/passphrase"
	"github.com/endophage/gotuf/data"
)

// KeyFileStore persists and manages private keys on disk
type KeyFileStore struct {
	sync.Mutex
	SimpleFileStore
	passphrase.Retriever
	cachedKeys map[string]*cachedKey
}

// KeyMemoryStore manages private keys in memory
type KeyMemoryStore struct {
	sync.Mutex
	MemoryFileStore
	passphrase.Retriever
	cachedKeys map[string]*cachedKey
}

// NewKeyFileStore returns a new KeyFileStore creating a private directory to
// hold the keys.
func NewKeyFileStore(baseDir string, passphraseRetriever passphrase.Retriever) (*KeyFileStore, error) {
	fileStore, err := NewPrivateSimpleFileStore(baseDir, keyExtension)
	if err != nil {
		return nil, err
	}
	cachedKeys := make(map[string]*cachedKey)

	return &KeyFileStore{SimpleFileStore: *fileStore,
		Retriever:  passphraseRetriever,
		cachedKeys: cachedKeys}, nil
}

// AddKey stores the contents of a PEM-encoded private key as a PEM block
func (s *KeyFileStore) AddKey(name, alias string, privKey data.PrivateKey) error {
	s.Lock()
	defer s.Unlock()
	return addKey(s, s.Retriever, s.cachedKeys, name, alias, privKey)
}

// GetKey returns the PrivateKey given a KeyID
func (s *KeyFileStore) GetKey(name string) (data.PrivateKey, string, error) {
	s.Lock()
	defer s.Unlock()
	return getKey(s, s.Retriever, s.cachedKeys, name)
}

// ListKeys returns a list of unique PublicKeys present on the KeyFileStore.
// There might be symlinks associating Certificate IDs to Public Keys, so this
// method only returns the IDs that aren't symlinks
func (s *KeyFileStore) ListKeys() map[string]string {
	return listKeys(s)
}

// RemoveKey removes the key from the keyfilestore
func (s *KeyFileStore) RemoveKey(name string) error {
	s.Lock()
	defer s.Unlock()
	return removeKey(s, s.cachedKeys, name)
}

// NewKeyMemoryStore returns a new KeyMemoryStore which holds keys in memory
func NewKeyMemoryStore(passphraseRetriever passphrase.Retriever) *KeyMemoryStore {
	memStore := NewMemoryFileStore()
	cachedKeys := make(map[string]*cachedKey)

	return &KeyMemoryStore{MemoryFileStore: *memStore,
		Retriever:  passphraseRetriever,
		cachedKeys: cachedKeys}
}

// AddKey stores the contents of a PEM-encoded private key as a PEM block
func (s *KeyMemoryStore) AddKey(name, alias string, privKey data.PrivateKey) error {
	s.Lock()
	defer s.Unlock()
	return addKey(s, s.Retriever, s.cachedKeys, name, alias, privKey)
}

// GetKey returns the PrivateKey given a KeyID
func (s *KeyMemoryStore) GetKey(name string) (data.PrivateKey, string, error) {
	s.Lock()
	defer s.Unlock()
	return getKey(s, s.Retriever, s.cachedKeys, name)
}

// ListKeys returns a list of unique PublicKeys present on the KeyFileStore.
// There might be symlinks associating Certificate IDs to Public Keys, so this
// method only returns the IDs that aren't symlinks
func (s *KeyMemoryStore) ListKeys() map[string]string {
	return listKeys(s)
}

// RemoveKey removes the key from the keystore
func (s *KeyMemoryStore) RemoveKey(name string) error {
	s.Lock()
	defer s.Unlock()
	return removeKey(s, s.cachedKeys, name)
}

func addKey(s LimitedFileStore, passphraseRetriever passphrase.Retriever, cachedKeys map[string]*cachedKey, name, alias string, privKey data.PrivateKey) error {
	pemPrivKey, err := KeyToPEM(privKey)
	if err != nil {
		return err
	}

	attempts := 0
	chosenPassphrase := ""
	giveup := false
	for {
		chosenPassphrase, giveup, err = passphraseRetriever(name, alias, true, attempts)
		if err != nil {
			attempts++
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

	if chosenPassphrase != "" {
		pemPrivKey, err = EncryptPrivateKey(privKey, chosenPassphrase)
		if err != nil {
			return err
		}
	}

	cachedKeys[name] = &cachedKey{alias: alias, key: privKey}
	return s.Add(name+"_"+alias, pemPrivKey)
}

func getKeyAlias(s LimitedFileStore, keyID string) (string, error) {
	files := s.ListFiles(true)
	name := strings.TrimSpace(strings.TrimSuffix(filepath.Base(keyID), filepath.Ext(keyID)))

	for _, file := range files {
		filename := filepath.Base(file)

		if strings.HasPrefix(filename, name) {
			aliasPlusDotKey := strings.TrimPrefix(filename, name+"_")
			retVal := strings.TrimSuffix(aliasPlusDotKey, "."+keyExtension)
			return retVal, nil
		}
	}

	return "", &ErrKeyNotFound{KeyID: keyID}
}

// GetKey returns the PrivateKey given a KeyID
func getKey(s LimitedFileStore, passphraseRetriever passphrase.Retriever, cachedKeys map[string]*cachedKey, name string) (data.PrivateKey, string, error) {
	cachedKeyEntry, ok := cachedKeys[name]
	if ok {
		return cachedKeyEntry.key, cachedKeyEntry.alias, nil
	}
	keyAlias, err := getKeyAlias(s, name)
	if err != nil {
		return nil, "", err
	}

	keyBytes, err := s.Get(name + "_" + keyAlias)
	if err != nil {
		return nil, "", err
	}

	var retErr error
	// See if the key is encrypted. If its encrypted we'll fail to parse the private key
	privKey, err := ParsePEMPrivateKey(keyBytes, "")
	if err != nil {
		// We need to decrypt the key, lets get a passphrase
		for attempts := 0; ; attempts++ {
			passphrase, giveup, err := passphraseRetriever(name, string(keyAlias), false, attempts)
			// Check if the passphrase retriever got an error or if it is telling us to give up
			if giveup || err != nil {
				return nil, "", ErrPasswordInvalid{}
			}
			if attempts > 10 {
				return nil, "", ErrAttemptsExceeded{}
			}

			// Try to convert PEM encoded bytes back to a PrivateKey using the passphrase
			privKey, err = ParsePEMPrivateKey(keyBytes, passphrase)
			if err != nil {
				retErr = ErrPasswordInvalid{}
			} else {
				// We managed to parse the PrivateKey. We've succeeded!
				retErr = nil
				break
			}
		}
	}
	if retErr != nil {
		return nil, "", retErr
	}
	cachedKeys[name] = &cachedKey{alias: keyAlias, key: privKey}
	return privKey, keyAlias, nil
}

// ListKeys returns a map of unique PublicKeys present on the KeyFileStore and
// their corresponding aliases.
// There might be symlinks associating Certificate IDs to Public Keys, so this
// method only returns the IDs that aren't symlinks
func listKeys(s LimitedFileStore) map[string]string {
	keyIDMap := make(map[string]string)

	for _, f := range s.ListFiles(false) {
		keyIDFull := strings.TrimSpace(strings.TrimSuffix(f, filepath.Ext(f)))
		keyID := keyIDFull[:strings.LastIndex(keyIDFull, "_")]
		keyAlias := keyIDFull[strings.LastIndex(keyIDFull, "_")+1:]
		keyIDMap[keyID] = keyAlias
	}
	return keyIDMap
}

// RemoveKey removes the key from the keyfilestore
func removeKey(s LimitedFileStore, cachedKeys map[string]*cachedKey, name string) error {
	keyAlias, err := getKeyAlias(s, name)
	if err != nil {
		return err
	}

	delete(cachedKeys, name)

	return s.Remove(name + "_" + keyAlias)
}
