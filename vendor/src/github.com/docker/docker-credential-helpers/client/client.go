package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/docker-credential-helpers/credentials"
)

// Store uses an external program to save credentials.
func Store(program ProgramFunc, credentials *credentials.Credentials) error {
	cmd := program("store")

	buffer := new(bytes.Buffer)
	if err := json.NewEncoder(buffer).Encode(credentials); err != nil {
		return err
	}
	cmd.Input(buffer)

	out, err := cmd.Output()
	if err != nil {
		t := strings.TrimSpace(string(out))
		return fmt.Errorf("error storing credentials - err: %v, out: `%s`", err, t)
	}

	return nil
}

// Get executes an external program to get the credentials from a native store.
func Get(program ProgramFunc, serverURL string) (*credentials.Credentials, error) {
	cmd := program("get")
	cmd.Input(strings.NewReader(serverURL))

	out, err := cmd.Output()
	if err != nil {
		t := strings.TrimSpace(string(out))

		if credentials.IsErrCredentialsNotFoundMessage(t) {
			return nil, credentials.NewErrCredentialsNotFound()
		}

		return nil, fmt.Errorf("error getting credentials - err: %v, out: `%s`", err, t)
	}

	resp := &credentials.Credentials{
		ServerURL: serverURL,
	}

	if err := json.NewDecoder(bytes.NewReader(out)).Decode(resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// Erase executes a program to remove the server credentails from the native store.
func Erase(program ProgramFunc, serverURL string) error {
	cmd := program("erase")
	cmd.Input(strings.NewReader(serverURL))

	out, err := cmd.Output()
	if err != nil {
		t := strings.TrimSpace(string(out))
		return fmt.Errorf("error erasing credentials - err: %v, out: `%s`", err, t)
	}

	return nil
}
