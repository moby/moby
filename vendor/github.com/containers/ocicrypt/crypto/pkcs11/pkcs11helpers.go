// +build cgo

/*
   Copyright The ocicrypt Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package pkcs11

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/miekg/pkcs11"
	"github.com/pkg/errors"
	pkcs11uri "github.com/stefanberger/go-pkcs11uri"
)

var (
	// OAEPLabel defines the label we use for OAEP encryption; this cannot be changed
	OAEPLabel = []byte("")
	// OAEPDefaultHash defines the default hash used for OAEP encryption; this cannot be changed
	OAEPDefaultHash = "sha1"

	// OAEPSha1Params describes the OAEP parameters with sha1 hash algorithm; needed by SoftHSM
	OAEPSha1Params = &pkcs11.OAEPParams{
		HashAlg:    pkcs11.CKM_SHA_1,
		MGF:        pkcs11.CKG_MGF1_SHA1,
		SourceType: pkcs11.CKZ_DATA_SPECIFIED,
		SourceData: OAEPLabel,
	}
	// OAEPSha256Params describes the OAEP parameters with sha256 hash algorithm
	OAEPSha256Params = &pkcs11.OAEPParams{
		HashAlg:    pkcs11.CKM_SHA256,
		MGF:        pkcs11.CKG_MGF1_SHA256,
		SourceType: pkcs11.CKZ_DATA_SPECIFIED,
		SourceData: OAEPLabel,
	}
)

// rsaPublicEncryptOAEP encrypts the given plaintext with the given *rsa.PublicKey; the
// environment variable OCICRYPT_OAEP_HASHALG can be set to 'sha1' to force usage of sha1 for OAEP (SoftHSM).
// This function is needed by clients who are using a public key file for pkcs11 encryption
func rsaPublicEncryptOAEP(pubKey *rsa.PublicKey, plaintext []byte) ([]byte, string, error) {
	var (
		hashfunc hash.Hash
		hashalg  string
	)

	oaephash := os.Getenv("OCICRYPT_OAEP_HASHALG")
	// The default is 'sha1'
	switch strings.ToLower(oaephash) {
	case "sha1", "":
		hashfunc = sha1.New()
		hashalg = "sha1"
	case "sha256":
		hashfunc = sha256.New()
		hashalg = "sha256"
	default:
		return nil, "", errors.Errorf("Unsupported OAEP hash '%s'", oaephash)
	}
	ciphertext, err := rsa.EncryptOAEP(hashfunc, rand.Reader, pubKey, plaintext, OAEPLabel)
	if err != nil {
		return nil, "", errors.Wrapf(err, "rss.EncryptOAEP failed")
	}

	return ciphertext, hashalg, nil
}

// pkcs11UriGetLoginParameters gets the parameters necessary for login from the Pkcs11URI
// PIN and module are mandatory; slot-id is optional and if not found -1 will be returned
// For a privateKeyOperation a PIN is required and if none is given, this function will return an error
func pkcs11UriGetLoginParameters(p11uri *pkcs11uri.Pkcs11URI, privateKeyOperation bool) (string, string, int64, error) {
	var (
		pin string
		err error
	)
	if privateKeyOperation {
		if !p11uri.HasPIN() {
			return "", "", 0, errors.New("Missing PIN for private key operation")
		}
	}
	// some devices require a PIN to find a *public* key object, others don't
	pin, _ = p11uri.GetPIN()

	module, err := p11uri.GetModule()
	if err != nil {
		return "", "", 0, errors.Wrap(err, "No module available in pkcs11 URI")
	}

	slotid := int64(-1)

	slot, ok := p11uri.GetPathAttribute("slot-id", false)
	if ok {
		slotid, err = strconv.ParseInt(slot, 10, 64)
		if err != nil {
			return "", "", 0, errors.Wrap(err, "slot-id is not a valid number")
		}
		if slotid < 0 {
			return "", "", 0, fmt.Errorf("slot-id is a negative number")
		}
		if uint64(slotid) > 0xffffffff {
			return "", "", 0, fmt.Errorf("slot-id is larger than 32 bit")
		}
	}

	return pin, module, slotid, nil
}

// pkcs11UriGetKeyIdAndLabel gets the key label by retrieving the value of the 'object' attribute
func pkcs11UriGetKeyIdAndLabel(p11uri *pkcs11uri.Pkcs11URI) (string, string, error) {
	keyid, ok2 := p11uri.GetPathAttribute("id", false)
	label, ok1 := p11uri.GetPathAttribute("object", false)
	if !ok1 && !ok2 {
		return "", "", errors.New("Neither 'id' nor 'object' attributes were found in pkcs11 URI")
	}
	return keyid, label, nil
}

// pkcs11OpenSession opens a session with a pkcs11 device at the given slot and logs in with the given PIN
func pkcs11OpenSession(p11ctx *pkcs11.Ctx, slotid uint, pin string) (session pkcs11.SessionHandle, err error) {
	session, err = p11ctx.OpenSession(uint(slotid), pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		return 0, errors.Wrapf(err, "OpenSession to slot %d failed", slotid)
	}
	if len(pin) > 0 {
		err = p11ctx.Login(session, pkcs11.CKU_USER, pin)
		if err != nil {
			_ = p11ctx.CloseSession(session)
			return 0, errors.Wrap(err, "Could not login to device")
		}
	}
	return session, nil
}

// pkcs11UriLogin uses the given pkcs11 URI to select the pkcs11 module (share libary) and to get
// the PIN to use for login; if the URI contains a slot-id, the given slot-id will be used, otherwise
// one slot after the other will be attempted and the first one where login succeeds will be used
func pkcs11UriLogin(p11uri *pkcs11uri.Pkcs11URI, privateKeyOperation bool) (ctx *pkcs11.Ctx, session pkcs11.SessionHandle, err error) {
	pin, module, slotid, err := pkcs11UriGetLoginParameters(p11uri, privateKeyOperation)
	if err != nil {
		return nil, 0, err
	}

	p11ctx := pkcs11.New(module)
	if p11ctx == nil {
		return nil, 0, errors.New("Please check module path, input is: " + module)
	}

	err = p11ctx.Initialize()
	if err != nil {
		p11Err := err.(pkcs11.Error)
		if p11Err != pkcs11.CKR_CRYPTOKI_ALREADY_INITIALIZED {
			return nil, 0, errors.Wrap(err, "Initialize failed")
		}
	}

	if slotid >= 0 {
		session, err := pkcs11OpenSession(p11ctx, uint(slotid), pin)
		return p11ctx, session, err
	} else {
		slots, err := p11ctx.GetSlotList(true)
		if err != nil {
			return nil, 0, errors.Wrap(err, "GetSlotList failed")
		}

		tokenlabel, ok := p11uri.GetPathAttribute("token", false)
		if !ok {
			return nil, 0, errors.New("Missing 'token' attribute since 'slot-id' was not given")
		}

		for _, slot := range slots {
			ti, err := p11ctx.GetTokenInfo(slot)
			if err != nil || ti.Label != tokenlabel {
				continue
			}

			session, err = pkcs11OpenSession(p11ctx, slot, pin)
			if err == nil {
				return p11ctx, session, err
			}
		}
		if len(pin) > 0 {
			return nil, 0, errors.New("Could not create session to any slot and/or log in")
		}
		return nil, 0, errors.New("Could not create session to any slot")
	}
}

func pkcs11Logout(ctx *pkcs11.Ctx, session pkcs11.SessionHandle) {
	_ = ctx.Logout(session)
	_ = ctx.CloseSession(session)
	_ = ctx.Finalize()
	ctx.Destroy()
}

// findObject finds an object of the given class with the given keyid and/or label
func findObject(p11ctx *pkcs11.Ctx, session pkcs11.SessionHandle, class uint, keyid, label string) (pkcs11.ObjectHandle, error) {
	msg := ""

	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, class),
	}
	if len(label) > 0 {
		template = append(template, pkcs11.NewAttribute(pkcs11.CKA_LABEL, label))
		msg = fmt.Sprintf("label '%s'", label)
	}
	if len(keyid) > 0 {
		template = append(template, pkcs11.NewAttribute(pkcs11.CKA_ID, keyid))
		if len(msg) > 0 {
			msg += " and "
		}
		msg += url.PathEscape(keyid)
	}

	if err := p11ctx.FindObjectsInit(session, template); err != nil {
		return 0, errors.Wrap(err, "FindObjectsInit failed")
	}

	obj, _, err := p11ctx.FindObjects(session, 100)
	if err != nil {
		return 0, errors.Wrap(err, "FindObjects failed")
	}

	if err := p11ctx.FindObjectsFinal(session); err != nil {
		return 0, errors.Wrap(err, "FindObjectsFinal failed")
	}
	if len(obj) > 1 {
		return 0, errors.Errorf("There are too many (=%d) keys with %s", len(obj), msg)
	} else if len(obj) == 1 {
		return obj[0], nil
	}

	return 0, errors.Errorf("Could not find any object with %s", msg)
}

// publicEncryptOAEP uses a public key described by a pkcs11 URI to OAEP encrypt the given plaintext
func publicEncryptOAEP(pubKey *Pkcs11KeyFileObject, plaintext []byte) ([]byte, string, error) {
	oldenv, err := setEnvVars(pubKey.Uri.GetEnvMap())
	if err != nil {
		return nil, "", err
	}
	defer restoreEnv(oldenv)

	p11ctx, session, err := pkcs11UriLogin(pubKey.Uri, false)
	if err != nil {
		return nil, "", err
	}
	defer pkcs11Logout(p11ctx, session)

	keyid, label, err := pkcs11UriGetKeyIdAndLabel(pubKey.Uri)
	if err != nil {
		return nil, "", err
	}

	p11PubKey, err := findObject(p11ctx, session, pkcs11.CKO_PUBLIC_KEY, keyid, label)
	if err != nil {
		return nil, "", err
	}

	var hashalg string

	var oaep *pkcs11.OAEPParams
	oaephash := os.Getenv("OCICRYPT_OAEP_HASHALG")
	// the default is sha1
	switch strings.ToLower(oaephash) {
	case "sha1", "":
		oaep = OAEPSha1Params
		hashalg = "sha1"
	case "sha256":
		oaep = OAEPSha256Params
		hashalg = "sha256"
	default:
		return nil, "", errors.Errorf("Unsupported OAEP hash '%s'", oaephash)
	}

	err = p11ctx.EncryptInit(session, []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_RSA_PKCS_OAEP, oaep)}, p11PubKey)
	if err != nil {
		return nil, "", errors.Wrap(err, "EncryptInit error")
	}

	ciphertext, err := p11ctx.Encrypt(session, plaintext)
	if err != nil {
		return nil, "", errors.Wrap(err, "Encrypt failed")
	}
	return ciphertext, hashalg, nil
}

// privateDecryptOAEP uses a pkcs11 URI describing a private key to OAEP decrypt a ciphertext
func privateDecryptOAEP(privKeyObj *Pkcs11KeyFileObject, ciphertext []byte, hashalg string) ([]byte, error) {
	oldenv, err := setEnvVars(privKeyObj.Uri.GetEnvMap())
	if err != nil {
		return nil, err
	}
	defer restoreEnv(oldenv)

	p11ctx, session, err := pkcs11UriLogin(privKeyObj.Uri, true)
	if err != nil {
		return nil, err
	}
	defer pkcs11Logout(p11ctx, session)

	keyid, label, err := pkcs11UriGetKeyIdAndLabel(privKeyObj.Uri)
	if err != nil {
		return nil, err
	}

	p11PrivKey, err := findObject(p11ctx, session, pkcs11.CKO_PRIVATE_KEY, keyid, label)
	if err != nil {
		return nil, err
	}

	var oaep *pkcs11.OAEPParams

	// the default is sha1
	switch hashalg {
	case "sha1", "":
		oaep = OAEPSha1Params
	case "sha256":
		oaep = OAEPSha256Params
	default:
		return nil, errors.Errorf("Unsupported hash algorithm '%s' for decryption", hashalg)
	}

	err = p11ctx.DecryptInit(session, []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_RSA_PKCS_OAEP, oaep)}, p11PrivKey)
	if err != nil {
		return nil, errors.Wrapf(err, "DecryptInit failed")
	}
	plaintext, err := p11ctx.Decrypt(session, ciphertext)
	if err != nil {
		return nil, errors.Wrapf(err, "Decrypt failed")
	}
	return plaintext, err
}

//
// The following part deals with the JSON formatted message for multiple pkcs11 recipients
//

// Pkcs11Blob holds the encrypted blobs for all recipients; this is what we will put into the image's annotations
type Pkcs11Blob struct {
	Version    uint              `json:"version"`
	Recipients []Pkcs11Recipient `json:"recipients"`
}

// Pkcs11Recipient holds the b64-encoded and encrypted blob for a particular recipient
type Pkcs11Recipient struct {
	Version uint   `json:"version"`
	Blob    string `json:"blob"`
	Hash    string `json:"hash,omitempty"`
}

// EncryptMultiple encrypts for one or multiple pkcs11 devices; the public keys passed to this function
// may either be *rsa.PublicKey or *pkcs11uri.Pkcs11URI; the returned byte array is a JSON string of the
// following format:
// {
//   recipients: [  // recipient list
//     {
//        "version": 0,
//        "blob": <base64 encoded RSA OAEP encrypted blob>,
//        "hash": <hash used for OAEP other than 'sha256'>
//     } ,
//     {
//        "version": 0,
//        "blob": <base64 encoded RSA OAEP encrypted blob>,
//        "hash": <hash used for OAEP other than 'sha256'>
//     } ,
//     [...]
//   ]
// }
func EncryptMultiple(pubKeys []interface{}, data []byte) ([]byte, error) {
	var (
		ciphertext []byte
		err        error
		pkcs11blob Pkcs11Blob = Pkcs11Blob{Version: 0}
		hashalg    string
	)

	for _, pubKey := range pubKeys {
		switch pkey := pubKey.(type) {
		case *rsa.PublicKey:
			ciphertext, hashalg, err = rsaPublicEncryptOAEP(pkey, data)
		case *Pkcs11KeyFileObject:
			ciphertext, hashalg, err = publicEncryptOAEP(pkey, data)
		default:
			err = errors.Errorf("Unsupported key object type for pkcs11 public key")
		}
		if err != nil {
			return nil, err
		}

		if hashalg == OAEPDefaultHash {
			hashalg = ""
		}
		recipient := Pkcs11Recipient{
			Version: 0,
			Blob:    base64.StdEncoding.EncodeToString(ciphertext),
			Hash:    hashalg,
		}

		pkcs11blob.Recipients = append(pkcs11blob.Recipients, recipient)
	}
	return json.Marshal(&pkcs11blob)
}

// Decrypt tries to decrypt one of the recipients' blobs using a pkcs11 private key.
// The input pkcs11blobstr is a string with the following format:
// {
//   recipients: [  // recipient list
//     {
//        "version": 0,
//        "blob": <base64 encoded RSA OAEP encrypted blob>,
//        "hash": <hash used for OAEP other than 'sha256'>
//     } ,
//     {
//        "version": 0,
//        "blob": <base64 encoded RSA OAEP encrypted blob>,
//        "hash": <hash used for OAEP other than 'sha256'>
//     } ,
//     [...]
// }
func Decrypt(privKeyObjs []*Pkcs11KeyFileObject, pkcs11blobstr []byte) ([]byte, error) {
	pkcs11blob := Pkcs11Blob{}
	err := json.Unmarshal(pkcs11blobstr, &pkcs11blob)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not parse Pkcs11Blob")
	}
	switch pkcs11blob.Version {
	case 0:
		// latest supported version
	default:
		return nil, errors.Errorf("Found Pkcs11Blob with version %d but maximum supported version is 0.", pkcs11blob.Version)
	}
	// since we do trial and error, collect all encountered errors
	errs := ""

	for _, recipient := range pkcs11blob.Recipients {
		switch recipient.Version {
		case 0:
			// last supported version
		default:
			return nil, errors.Errorf("Found Pkcs11Recipient with version %d but maximum supported version is 0.", recipient.Version)
		}

		ciphertext, err := base64.StdEncoding.DecodeString(recipient.Blob)
		if err != nil || len(ciphertext) == 0 {
			// This should never happen... we skip over decoding issues
			errs += fmt.Sprintf("Base64 decoding failed: %s\n", err)
			continue
		}
		// try all keys until one works
		for _, privKeyObj := range privKeyObjs {
			plaintext, err := privateDecryptOAEP(privKeyObj, ciphertext, recipient.Hash)
			if err == nil {
				return plaintext, nil
			}
			if uri, err2 := privKeyObj.Uri.Format(); err2 == nil {
				errs += fmt.Sprintf("%s : %s\n", uri, err)
			} else {
				errs += fmt.Sprintf("%s\n", err)
			}
		}
	}

	return nil, errors.Errorf("Could not find a pkcs11 key for decryption:\n%s", errs)
}
