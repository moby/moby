package signed

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/endophage/gotuf/data"
	"github.com/endophage/gotuf/errors"
	"strings"
)

// Sign takes a data.Signed and a key, calculated and adds the signature
// to the data.Signed
func Sign(service CryptoService, s *data.Signed, keys ...data.PublicKey) error {
	logrus.Debugf("sign called with %d keys", len(keys))
	signatures := make([]data.Signature, 0, len(s.Signatures)+1)
	keyIDMemb := make(map[string]struct{})
	keyIDs := make([]string, 0, len(keys))

	for _, key := range keys {
		keyIDMemb[key.ID()] = struct{}{}
		keyIDs = append(keyIDs, key.ID())
	}
	logrus.Debugf("Generated list of signing IDs: %s", strings.Join(keyIDs, ", "))
	for _, sig := range s.Signatures {
		if _, ok := keyIDMemb[sig.KeyID]; ok {
			continue
		}
		signatures = append(signatures, sig)
	}
	newSigs, err := service.Sign(keyIDs, s.Signed)
	if err != nil {
		return err
	}
	if len(newSigs) < 1 {
		return errors.ErrInsufficientSignatures{
			Name: fmt.Sprint("Cryptoservice failed to produce any signatures for keys with IDs: %s", strings.Join(keyIDs, ", ")),
			Err:  nil,
		}
	}
	logrus.Debugf("appending %d new signatures", len(newSigs))
	s.Signatures = append(signatures, newSigs...)
	return nil
}
