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

package ocicrypt

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/term"
)

// GPGVersion enum representing the GPG client version to use.
type GPGVersion int

const (
	// GPGv2 signifies gpgv2+
	GPGv2 GPGVersion = iota
	// GPGv1 signifies gpgv1+
	GPGv1
	// GPGVersionUndetermined signifies gpg client version undetermined
	GPGVersionUndetermined
)

// GPGClient defines an interface for wrapping the gpg command line tools
type GPGClient interface {
	// ReadGPGPubRingFile gets the byte sequence of the gpg public keyring
	ReadGPGPubRingFile() ([]byte, error)
	// GetGPGPrivateKey gets the private key bytes of a keyid given a passphrase
	GetGPGPrivateKey(keyid uint64, passphrase string) ([]byte, error)
	// GetSecretKeyDetails gets the details of a secret key
	GetSecretKeyDetails(keyid uint64) ([]byte, bool, error)
	// GetKeyDetails gets the details of a public key
	GetKeyDetails(keyid uint64) ([]byte, bool, error)
	// ResolveRecipients resolves PGP key ids to user names
	ResolveRecipients([]string) []string
}

// gpgClient contains generic gpg client information
type gpgClient struct {
	gpgHomeDir string
}

// gpgv2Client is a gpg2 client
type gpgv2Client struct {
	gpgClient
}

// gpgv1Client is a gpg client
type gpgv1Client struct {
	gpgClient
}

// GuessGPGVersion guesses the version of gpg. Defaults to gpg2 if exists, if
// not defaults to regular gpg.
func GuessGPGVersion() GPGVersion {
	if err := exec.Command("gpg2", "--version").Run(); err == nil {
		return GPGv2
	} else if err := exec.Command("gpg", "--version").Run(); err == nil {
		return GPGv1
	} else {
		return GPGVersionUndetermined
	}
}

// NewGPGClient creates a new GPGClient object representing the given version
// and using the given home directory
func NewGPGClient(gpgVersion, gpgHomeDir string) (GPGClient, error) {
	v := new(GPGVersion)
	switch gpgVersion {
	case "v1":
		*v = GPGv1
	case "v2":
		*v = GPGv2
	default:
		v = nil
	}
	return newGPGClient(v, gpgHomeDir)
}

func newGPGClient(version *GPGVersion, homedir string) (GPGClient, error) {
	var gpgVersion GPGVersion
	if version != nil {
		gpgVersion = *version
	} else {
		gpgVersion = GuessGPGVersion()
	}

	switch gpgVersion {
	case GPGv1:
		return &gpgv1Client{
			gpgClient: gpgClient{gpgHomeDir: homedir},
		}, nil
	case GPGv2:
		return &gpgv2Client{
			gpgClient: gpgClient{gpgHomeDir: homedir},
		}, nil
	case GPGVersionUndetermined:
		return nil, fmt.Errorf("unable to determine GPG version")
	default:
		return nil, fmt.Errorf("unhandled case: NewGPGClient")
	}
}

// GetGPGPrivateKey gets the bytes of a specified keyid, supplying a passphrase
func (gc *gpgv2Client) GetGPGPrivateKey(keyid uint64, passphrase string) ([]byte, error) {
	var args []string

	if gc.gpgHomeDir != "" {
		args = append(args, []string{"--homedir", gc.gpgHomeDir}...)
	}

	rfile, wfile, err := os.Pipe()
	if err != nil {
		return nil, errors.Wrapf(err, "could not create pipe")
	}
	defer func() {
		rfile.Close()
		wfile.Close()
	}()
	// fill pipe in background
	go func(passphrase string) {
		_, _ = wfile.Write([]byte(passphrase))
		wfile.Close()
	}(passphrase)

	args = append(args, []string{"--pinentry-mode", "loopback", "--batch", "--passphrase-fd", fmt.Sprintf("%d", 3), "--export-secret-key", fmt.Sprintf("0x%x", keyid)}...)

	cmd := exec.Command("gpg2", args...)
	cmd.ExtraFiles = []*os.File{rfile}

	return runGPGGetOutput(cmd)
}

// ReadGPGPubRingFile reads the GPG public key ring file
func (gc *gpgv2Client) ReadGPGPubRingFile() ([]byte, error) {
	var args []string

	if gc.gpgHomeDir != "" {
		args = append(args, []string{"--homedir", gc.gpgHomeDir}...)
	}
	args = append(args, []string{"--batch", "--export"}...)

	cmd := exec.Command("gpg2", args...)

	return runGPGGetOutput(cmd)
}

func (gc *gpgv2Client) getKeyDetails(option string, keyid uint64) ([]byte, bool, error) {
	var args []string

	if gc.gpgHomeDir != "" {
		args = []string{"--homedir", gc.gpgHomeDir}
	}
	args = append(args, option, fmt.Sprintf("0x%x", keyid))

	cmd := exec.Command("gpg2", args...)

	keydata, err := runGPGGetOutput(cmd)
	return keydata, err == nil, err
}

// GetSecretKeyDetails retrieves the secret key details of key with keyid.
// returns a byte array of the details and a bool if the key exists
func (gc *gpgv2Client) GetSecretKeyDetails(keyid uint64) ([]byte, bool, error) {
	return gc.getKeyDetails("-K", keyid)
}

// GetKeyDetails retrieves the public key details of key with keyid.
// returns a byte array of the details and a bool if the key exists
func (gc *gpgv2Client) GetKeyDetails(keyid uint64) ([]byte, bool, error) {
	return gc.getKeyDetails("-k", keyid)
}

// ResolveRecipients converts PGP keyids to email addresses, if possible
func (gc *gpgv2Client) ResolveRecipients(recipients []string) []string {
	return resolveRecipients(gc, recipients)
}

// GetGPGPrivateKey gets the bytes of a specified keyid, supplying a passphrase
func (gc *gpgv1Client) GetGPGPrivateKey(keyid uint64, _ string) ([]byte, error) {
	var args []string

	if gc.gpgHomeDir != "" {
		args = append(args, []string{"--homedir", gc.gpgHomeDir}...)
	}
	args = append(args, []string{"--batch", "--export-secret-key", fmt.Sprintf("0x%x", keyid)}...)

	cmd := exec.Command("gpg", args...)

	return runGPGGetOutput(cmd)
}

// ReadGPGPubRingFile reads the GPG public key ring file
func (gc *gpgv1Client) ReadGPGPubRingFile() ([]byte, error) {
	var args []string

	if gc.gpgHomeDir != "" {
		args = append(args, []string{"--homedir", gc.gpgHomeDir}...)
	}
	args = append(args, []string{"--batch", "--export"}...)

	cmd := exec.Command("gpg", args...)

	return runGPGGetOutput(cmd)
}

func (gc *gpgv1Client) getKeyDetails(option string, keyid uint64) ([]byte, bool, error) {
	var args []string

	if gc.gpgHomeDir != "" {
		args = []string{"--homedir", gc.gpgHomeDir}
	}
	args = append(args, option, fmt.Sprintf("0x%x", keyid))

	cmd := exec.Command("gpg", args...)

	keydata, err := runGPGGetOutput(cmd)

	return keydata, err == nil, err
}

// GetSecretKeyDetails retrieves the secret key details of key with keyid.
// returns a byte array of the details and a bool if the key exists
func (gc *gpgv1Client) GetSecretKeyDetails(keyid uint64) ([]byte, bool, error) {
	return gc.getKeyDetails("-K", keyid)
}

// GetKeyDetails retrieves the public key details of key with keyid.
// returns a byte array of the details and a bool if the key exists
func (gc *gpgv1Client) GetKeyDetails(keyid uint64) ([]byte, bool, error) {
	return gc.getKeyDetails("-k", keyid)
}

// ResolveRecipients converts PGP keyids to email addresses, if possible
func (gc *gpgv1Client) ResolveRecipients(recipients []string) []string {
	return resolveRecipients(gc, recipients)
}

// runGPGGetOutput runs the GPG commandline and returns stdout as byte array
// and any stderr in the error
func runGPGGetOutput(cmd *exec.Cmd) ([]byte, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	stdoutstr, err2 := ioutil.ReadAll(stdout)
	stderrstr, _ := ioutil.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("error from %s: %s", cmd.Path, string(stderrstr))
	}

	return stdoutstr, err2
}

// resolveRecipients walks the list of recipients and attempts to convert
// all keyIds to email addresses; if something goes wrong during the
// conversion of a recipient, the original string is returned for that
// recpient
func resolveRecipients(gc GPGClient, recipients []string) []string {
	var result []string

	for _, recipient := range recipients {
		keyID, err := strconv.ParseUint(recipient, 0, 64)
		if err != nil {
			result = append(result, recipient)
		} else {
			details, found, _ := gc.GetKeyDetails(keyID)
			if !found {
				result = append(result, recipient)
			} else {
				email := extractEmailFromDetails(details)
				if email == "" {
					result = append(result, recipient)
				} else {
					result = append(result, email)
				}
			}
		}
	}
	return result
}

var emailPattern = regexp.MustCompile(`uid\s+\[.*\]\s.*\s<(?P<email>.+)>`)

func extractEmailFromDetails(details []byte) string {
	loc := emailPattern.FindSubmatchIndex(details)
	if len(loc) == 0 {
		return ""
	}
	return string(emailPattern.Expand(nil, []byte("$email"), details, loc))
}

// uint64ToStringArray converts an array of uint64's to an array of strings
// by applying a format string to each uint64
func uint64ToStringArray(format string, in []uint64) []string {
	var ret []string

	for _, v := range in {
		ret = append(ret, fmt.Sprintf(format, v))
	}
	return ret
}

// GPGGetPrivateKey walks the list of layerInfos and tries to decrypt the
// wrapped symmetric keys. For this it determines whether a private key is
// in the GPGVault or on this system and prompts for the passwords for those
// that are available. If we do not find a private key on the system for
// getting to the symmetric key of a layer then an error is generated.
func GPGGetPrivateKey(descs []ocispec.Descriptor, gpgClient GPGClient, gpgVault GPGVault, mustFindKey bool) (gpgPrivKeys [][]byte, gpgPrivKeysPwds [][]byte, err error) {
	// PrivateKeyData describes a private key
	type PrivateKeyData struct {
		KeyData         []byte
		KeyDataPassword []byte
	}
	var pkd PrivateKeyData
	keyIDPasswordMap := make(map[uint64]PrivateKeyData)

	for _, desc := range descs {
		for scheme, b64pgpPackets := range GetWrappedKeysMap(desc) {
			if scheme != "pgp" {
				continue
			}
			keywrapper := GetKeyWrapper(scheme)
			if keywrapper == nil {
				return nil, nil, errors.Errorf("could not get KeyWrapper for %s\n", scheme)
			}
			keyIds, err := keywrapper.GetKeyIdsFromPacket(b64pgpPackets)
			if err != nil {
				return nil, nil, err
			}

			found := false
			for _, keyid := range keyIds {
				// do we have this key? -- first check the vault
				if gpgVault != nil {
					_, keydata := gpgVault.GetGPGPrivateKey(keyid)
					if len(keydata) > 0 {
						pkd = PrivateKeyData{
							KeyData:         keydata,
							KeyDataPassword: nil, // password not supported in this case
						}
						keyIDPasswordMap[keyid] = pkd
						found = true
						break
					}
				} else if gpgClient != nil {
					// check the local system's gpg installation
					keyinfo, haveKey, _ := gpgClient.GetSecretKeyDetails(keyid)
					// this may fail if the key is not here; we ignore the error
					if !haveKey {
						// key not on this system
						continue
					}

					_, found = keyIDPasswordMap[keyid]
					if !found {
						fmt.Printf("Passphrase required for Key id 0x%x: \n%v", keyid, string(keyinfo))
						fmt.Printf("Enter passphrase for key with Id 0x%x: ", keyid)

						password, err := term.ReadPassword(int(os.Stdin.Fd()))
						fmt.Printf("\n")
						if err != nil {
							return nil, nil, err
						}
						keydata, err := gpgClient.GetGPGPrivateKey(keyid, string(password))
						if err != nil {
							return nil, nil, err
						}
						pkd = PrivateKeyData{
							KeyData:         keydata,
							KeyDataPassword: password,
						}
						keyIDPasswordMap[keyid] = pkd
						found = true
					}
					break
				} else {
					return nil, nil, errors.New("no GPGVault or GPGClient passed")
				}
			}
			if !found && len(b64pgpPackets) > 0 && mustFindKey {
				ids := uint64ToStringArray("0x%x", keyIds)

				return nil, nil, errors.Errorf("missing key for decryption of layer %x of %s. Need one of the following keys: %s", desc.Digest, desc.Platform, strings.Join(ids, ", "))
			}
		}
	}

	for _, pkd := range keyIDPasswordMap {
		gpgPrivKeys = append(gpgPrivKeys, pkd.KeyData)
		gpgPrivKeysPwds = append(gpgPrivKeysPwds, pkd.KeyDataPassword)
	}

	return gpgPrivKeys, gpgPrivKeysPwds, nil
}
