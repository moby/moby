// Package passphrase is a utility function for managing passphrase
// for TUF and Notary keys.
package passphrase

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"path/filepath"

	"github.com/docker/docker/pkg/term"
)

// Retriever is a callback function that should retrieve a passphrase
// for a given named key. If it should be treated as new passphrase (e.g. with
// confirmation), createNew will be true. Attempts is passed in so that implementers
// decide how many chances to give to a human, for example.
type Retriever func(keyName, alias string, createNew bool, attempts int) (passphrase string, giveup bool, err error)

const (
	idBytesToDisplay            = 7
	tufRootAlias                = "root"
	tufTargetsAlias             = "targets"
	tufSnapshotAlias            = "snapshot"
	tufRootKeyGenerationWarning = `You are about to create a new root signing key passphrase. This passphrase
will be used to protect the most sensitive key in your signing system. Please
choose a long, complex passphrase and be careful to keep the password and the
key file itself secure and backed up. It is highly recommended that you use a
password manager to generate the passphrase and keep it safe. There will be no
way to recover this key. You can find the key in your config directory.`
)

var (
	// ErrTooShort is returned if the passphrase entered for a new key is
	// below the minimum length
	ErrTooShort = errors.New("Passphrase too short")

	// ErrDontMatch is returned if the two entered passphrases don't match.
	// new key is below the minimum length
	ErrDontMatch = errors.New("The entered passphrases do not match")

	// ErrTooManyAttempts is returned if the maximum number of passphrase
	// entry attempts is reached.
	ErrTooManyAttempts = errors.New("Too many attempts")
)

// PromptRetriever returns a new Retriever which will provide a prompt on stdin
// and stdout to retrieve a passphrase. The passphrase will be cached such that
// subsequent prompts will produce the same passphrase.
func PromptRetriever() Retriever {
	return PromptRetrieverWithInOut(os.Stdin, os.Stdout, nil)
}

// PromptRetrieverWithInOut returns a new Retriever which will provide a
// prompt using the given in and out readers. The passphrase will be cached
// such that subsequent prompts will produce the same passphrase.
// aliasMap can be used to specify display names for TUF key aliases. If aliasMap
// is nil, a sensible default will be used.
func PromptRetrieverWithInOut(in io.Reader, out io.Writer, aliasMap map[string]string) Retriever {
	userEnteredTargetsSnapshotsPass := false
	targetsSnapshotsPass := ""
	userEnteredRootsPass := false
	rootsPass := ""

	return func(keyName string, alias string, createNew bool, numAttempts int) (string, bool, error) {
		if alias == tufRootAlias && createNew && numAttempts == 0 {
			fmt.Fprintln(out, tufRootKeyGenerationWarning)
		}
		if numAttempts > 0 {
			if !createNew {
				fmt.Fprintln(out, "Passphrase incorrect. Please retry.")
			}
		}

		// Figure out if we should display a different string for this alias
		displayAlias := alias
		if aliasMap != nil {
			if val, ok := aliasMap[alias]; ok {
				displayAlias = val
			}

		}

		// First, check if we have a password cached for this alias.
		if numAttempts == 0 {
			if userEnteredTargetsSnapshotsPass && (alias == tufSnapshotAlias || alias == tufTargetsAlias) {
				return targetsSnapshotsPass, false, nil
			}
			if userEnteredRootsPass && (alias == "root") {
				return rootsPass, false, nil
			}
		}

		if numAttempts > 3 && !createNew {
			return "", true, ErrTooManyAttempts
		}

		state, err := term.SaveState(0)
		if err != nil {
			return "", false, err
		}
		term.DisableEcho(0, state)
		defer term.RestoreTerminal(0, state)

		stdin := bufio.NewReader(in)

		indexOfLastSeparator := strings.LastIndex(keyName, string(filepath.Separator))
		if indexOfLastSeparator == -1 {
			indexOfLastSeparator = 0
		}

		var shortName string
		if len(keyName) > indexOfLastSeparator+idBytesToDisplay {
			if indexOfLastSeparator > 0 {
				keyNamePrefix := keyName[:indexOfLastSeparator]
				keyNameID := keyName[indexOfLastSeparator+1 : indexOfLastSeparator+idBytesToDisplay+1]
				shortName = keyNameID + " (" + keyNamePrefix + ")"
			} else {
				shortName = keyName[indexOfLastSeparator : indexOfLastSeparator+idBytesToDisplay]
			}
		}

		withID := fmt.Sprintf(" with ID %s", shortName)
		if shortName == "" {
			withID = ""
		}

		if createNew {
			fmt.Fprintf(out, "Enter passphrase for new %s key%s: ", displayAlias, withID)
		} else if displayAlias == "yubikey" {
			fmt.Fprintf(out, "Enter the %s for the attached Yubikey: ", keyName)
		} else {
			fmt.Fprintf(out, "Enter passphrase for %s key%s: ", displayAlias, withID)
		}

		passphrase, err := stdin.ReadBytes('\n')
		fmt.Fprintln(out)
		if err != nil {
			return "", false, err
		}

		retPass := strings.TrimSpace(string(passphrase))

		if !createNew {
			if alias == tufSnapshotAlias || alias == tufTargetsAlias {
				userEnteredTargetsSnapshotsPass = true
				targetsSnapshotsPass = retPass
			}
			if alias == tufRootAlias {
				userEnteredRootsPass = true
				rootsPass = retPass
			}
			return retPass, false, nil
		}

		if len(retPass) < 8 {
			fmt.Fprintln(out, "Passphrase is too short. Please use a password manager to generate and store a good random passphrase.")
			return "", false, ErrTooShort
		}

		fmt.Fprintf(out, "Repeat passphrase for new %s key%s: ", displayAlias, withID)
		confirmation, err := stdin.ReadBytes('\n')
		fmt.Fprintln(out)
		if err != nil {
			return "", false, err
		}
		confirmationStr := strings.TrimSpace(string(confirmation))

		if retPass != confirmationStr {
			fmt.Fprintln(out, "Passphrases do not match. Please retry.")
			return "", false, ErrDontMatch
		}

		if alias == tufSnapshotAlias || alias == tufTargetsAlias {
			userEnteredTargetsSnapshotsPass = true
			targetsSnapshotsPass = retPass
		}
		if alias == tufRootAlias {
			userEnteredRootsPass = true
			rootsPass = retPass
		}

		return retPass, false, nil
	}
}

// ConstantRetriever returns a new Retriever which will return a constant string
// as a passphrase.
func ConstantRetriever(constantPassphrase string) Retriever {
	return func(k, a string, c bool, n int) (string, bool, error) {
		return constantPassphrase, false, nil
	}
}
