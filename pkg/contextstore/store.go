package contextstore

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const configFileName = "config.json"

// Store provides a context store for easily remembering endpoints configuration
type Store interface {
	GetCurrentContext() string
	SetCurrentContext(name string) error
	ListContexts() (map[string]ContextMetadata, error)
	CreateOrUpdateContext(name string, meta ContextMetadata) error
	RemoveContext(name string) error
	GetContextMetadata(name string) (ContextMetadata, error)
	ResetContextTLSMaterial(name string, data *ContextTLSData) error
	ResetContextEndpointTLSMaterial(contextName string, endpointName string, data *EndpointTLSData) error
	ListContextTLSFiles(name string) (map[string]EndpointFiles, error)
	GetContextTLSData(contextName, endpointName, fileName string) ([]byte, error)
}

// EndpointMetadata contains metadata about an endpoint
type EndpointMetadata map[string]interface{}

// ContextMetadata contains metadata about a context and its endpoints
type ContextMetadata struct {
	Metadata  map[string]interface{}      `json:"metadata,omitempty"`
	Endpoints map[string]EndpointMetadata `json:"endpoints,omitempty"`
}

// EndpointTLSData represents tls data for a given endpoint
type EndpointTLSData struct {
	Files map[string][]byte
}

// ContextTLSData represents tls data for a whole context
type ContextTLSData struct {
	Endpoints map[string]EndpointTLSData
}

// NewStore creates a store from a given directory.
// If the directory does not exist or is empty, initialize it
func NewStore(dir string) (Store, error) {
	metaRoot := filepath.Join(dir, metadataDir)
	tlsRoot := filepath.Join(dir, tlsDir)
	configFile := filepath.Join(dir, configFileName)
	if err := os.MkdirAll(metaRoot, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tlsRoot, 0700); err != nil {
		return nil, err
	}
	_, err := os.Stat(configFile)

	switch {
	case os.IsNotExist(err):
		//create default file
		if err := ioutil.WriteFile(configFile, []byte("{}"), 0644); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	}

	configBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	var cfg config
	if err := json.Unmarshal(configBytes, &cfg); err != nil {
		return nil, err
	}
	return &store{
		configFile:     configFile,
		currentContext: cfg.CurrentContext,
		meta: &metadataStore{
			root: metaRoot,
		},
		tls: &tlsStore{
			root: tlsRoot,
		},
	}, nil
}

type store struct {
	configFile     string
	currentContext string
	meta           *metadataStore
	tls            *tlsStore
}

func (s *store) GetCurrentContext() string {
	return s.currentContext
}

func (s *store) SetCurrentContext(name string) error {
	cfg := config{CurrentContext: name}
	configBytes, err := json.Marshal(&cfg)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(s.configFile, configBytes, 0644); err != nil {
		return err
	}
	s.currentContext = name
	return nil
}

func (s *store) ListContexts() (map[string]ContextMetadata, error) {
	return s.meta.list()
}

func (s *store) CreateOrUpdateContext(name string, meta ContextMetadata) error {
	return s.meta.createOrUpdate(name, meta)
}

func (s *store) RemoveContext(name string) error {
	if err := s.meta.remove(name); err != nil {
		return err
	}
	return s.tls.removeAllContextData(name)
}

func (s *store) GetContextMetadata(name string) (ContextMetadata, error) {
	return s.meta.get(name)
}

func (s *store) ResetContextTLSMaterial(name string, data *ContextTLSData) error {
	if err := s.tls.removeAllContextData(name); err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	for ep, files := range data.Endpoints {
		for fileName, data := range files.Files {
			if err := s.tls.createOrUpdate(name, ep, fileName, data); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *store) ResetContextEndpointTLSMaterial(contextName string, endpointName string, data *EndpointTLSData) error {
	if err := s.tls.removeAllEndpointData(contextName, endpointName); err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	for fileName, data := range data.Files {
		if err := s.tls.createOrUpdate(contextName, endpointName, fileName, data); err != nil {
			return err
		}
	}
	return nil
}

func (s *store) ListContextTLSFiles(name string) (map[string]EndpointFiles, error) {
	return s.tls.listContextData(name)
}

func (s *store) GetContextTLSData(contextName, endpointName, fileName string) ([]byte, error) {
	return s.tls.getData(contextName, endpointName, fileName)
}

// Export exports an existing namespace into a stream
func Export(name string, s Store) io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		tw := tar.NewWriter(writer)
		defer tw.Close()
		defer writer.Close()
		meta, err := s.GetContextMetadata(name)
		if err != nil {
			writer.CloseWithError(err)
			return
		}
		metaBytes, err := json.Marshal(&meta)
		if err != nil {
			writer.CloseWithError(err)
			return
		}
		if err = tw.WriteHeader(&tar.Header{
			Name: metaFile,
			Mode: 0644,
			Size: int64(len(metaBytes)),
		}); err != nil {
			writer.CloseWithError(err)
			return
		}
		if _, err = tw.Write(metaBytes); err != nil {
			writer.CloseWithError(err)
			return
		}
		tlsFiles, err := s.ListContextTLSFiles(name)
		if err != nil {
			writer.CloseWithError(err)
			return
		}
		for endpointName, endpointFiles := range tlsFiles {
			for _, fileName := range endpointFiles {
				data, err := s.GetContextTLSData(name, endpointName, fileName)
				if err != nil {
					writer.CloseWithError(err)
					return
				}
				if err = tw.WriteHeader(&tar.Header{
					Name: path.Join("tls", endpointName, fileName),
					Mode: 0600,
					Size: int64(len(data)),
				}); err != nil {
					writer.CloseWithError(err)
					return
				}
				if _, err = tw.Write(data); err != nil {
					writer.CloseWithError(err)
					return
				}
			}
		}
	}()
	return reader
}

// Import imports an exported context into a store
func Import(name string, s Store, reader io.Reader) error {
	tr := tar.NewReader(reader)
	tlsData := ContextTLSData{
		Endpoints: map[string]EndpointTLSData{},
	}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if hdr.Name == metaFile {
			data, err := ioutil.ReadAll(tr)
			if err != nil {
				return err
			}
			var meta ContextMetadata
			if err := json.Unmarshal(data, &meta); err != nil {
				return err
			}
			if err := s.CreateOrUpdateContext(name, meta); err != nil {
				return err
			}
		} else if strings.HasPrefix(hdr.Name, "tls/") {
			relative := strings.TrimPrefix(hdr.Name, "tls/")
			parts := strings.SplitN(relative, "/", 2)
			if len(parts) != 2 {
				return errors.New("archive format is invalid")
			}
			endpointName := parts[0]
			fileName := parts[1]
			data, err := ioutil.ReadAll(tr)
			if err != nil {
				return err
			}
			if _, ok := tlsData.Endpoints[endpointName]; !ok {
				tlsData.Endpoints[endpointName] = EndpointTLSData{
					Files: map[string][]byte{},
				}
			}
			tlsData.Endpoints[endpointName].Files[fileName] = data
		}
	}
	return s.ResetContextTLSMaterial(name, &tlsData)
}

type config struct {
	CurrentContext string `json:"current_context,omitempty"`
}
