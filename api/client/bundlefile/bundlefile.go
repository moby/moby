// +build experimental

package bundlefile

import (
	"encoding/json"
	"io"
	"os"
)

// Bundlefile stores the contents of a bundlefile
type Bundlefile struct {
	Version  string
	Services map[string]Service
}

// Service is a service from a bundlefile
type Service struct {
	Image      string
	Command    []string          `json:",omitempty"`
	Args       []string          `json:",omitempty"`
	Env        []string          `json:",omitempty"`
	Labels     map[string]string `json:",omitempty"`
	Ports      []Port            `json:",omitempty"`
	WorkingDir *string           `json:",omitempty"`
	User       *string           `json:",omitempty"`
	Networks   []string          `json:",omitempty"`
}

// Port is a port as defined in a bundlefile
type Port struct {
	Protocol string
	Port     uint32
}

// LoadFile loads a bundlefile from a path to the file
func LoadFile(path string) (*Bundlefile, error) {
	reader, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	bundlefile := &Bundlefile{}

	if err := json.NewDecoder(reader).Decode(bundlefile); err != nil {
		return nil, err
	}

	return bundlefile, err
}

// Print writes the contents of the bundlefile to the output writer
// as human readable json
func Print(out io.Writer, bundle *Bundlefile) error {
	bytes, err := json.MarshalIndent(*bundle, "", "    ")
	if err != nil {
		return err
	}

	_, err = out.Write(bytes)
	return err
}
