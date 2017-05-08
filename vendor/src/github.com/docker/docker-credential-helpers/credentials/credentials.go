package credentials

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Credentials holds the information shared between docker and the credentials store.
type Credentials struct {
	ServerURL string
	Username  string
	Secret    string
}

// Serve initializes the credentials helper and parses the action argument.
// This function is designed to be called from a command line interface.
// It uses os.Args[1] as the key for the action.
// It uses os.Stdin as input and os.Stdout as output.
// This function terminates the program with os.Exit(1) if there is an error.
func Serve(helper Helper) {
	var err error
	if len(os.Args) != 2 {
		err = fmt.Errorf("Usage: %s <store|get|erase>", os.Args[0])
	}

	if err == nil {
		err = HandleCommand(helper, os.Args[1], os.Stdin, os.Stdout)
	}

	if err != nil {
		fmt.Fprintf(os.Stdout, "%v\n", err)
		os.Exit(1)
	}
}

// HandleCommand uses a helper and a key to run a credential action.
func HandleCommand(helper Helper, key string, in io.Reader, out io.Writer) error {
	switch key {
	case "store":
		return Store(helper, in)
	case "get":
		return Get(helper, in, out)
	case "erase":
		return Erase(helper, in)
	}
	return fmt.Errorf("Unknown credential action `%s`", key)
}

// Store uses a helper and an input reader to save credentials.
// The reader must contain the JSON serialization of a Credentials struct.
func Store(helper Helper, reader io.Reader) error {
	scanner := bufio.NewScanner(reader)

	buffer := new(bytes.Buffer)
	for scanner.Scan() {
		buffer.Write(scanner.Bytes())
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return err
	}

	var creds Credentials
	if err := json.NewDecoder(buffer).Decode(&creds); err != nil {
		return err
	}

	return helper.Add(&creds)
}

// Get retrieves the credentials for a given server url.
// The reader must contain the server URL to search.
// The writer is used to write the JSON serialization of the credentials.
func Get(helper Helper, reader io.Reader, writer io.Writer) error {
	scanner := bufio.NewScanner(reader)

	buffer := new(bytes.Buffer)
	for scanner.Scan() {
		buffer.Write(scanner.Bytes())
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return err
	}

	serverURL := strings.TrimSpace(buffer.String())

	username, secret, err := helper.Get(serverURL)
	if err != nil {
		return err
	}

	resp := Credentials{
		Username: username,
		Secret:   secret,
	}

	buffer.Reset()
	if err := json.NewEncoder(buffer).Encode(resp); err != nil {
		return err
	}

	fmt.Fprint(writer, buffer.String())
	return nil
}

// Erase removes credentials from the store.
// The reader must contain the server URL to remove.
func Erase(helper Helper, reader io.Reader) error {
	scanner := bufio.NewScanner(reader)

	buffer := new(bytes.Buffer)
	for scanner.Scan() {
		buffer.Write(scanner.Bytes())
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return err
	}

	serverURL := strings.TrimSpace(buffer.String())

	return helper.Delete(serverURL)
}
