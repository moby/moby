package contextstore

import (
	"archive/tar"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
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
	GetContextMetadata(name string) (ContextMetadata, error)
	ResetContextTLSMaterial(name string, data *ContextTLSData) error
	ResetContextEndpointTLSMaterial(contextName string, endpointName string, data *EndpointTLSData) error
	ListContextTLSFiles(name string) (map[string]EndpointFiles, error)
	GetContextTLSData(contextName, endpointName, fileName string) ([]byte, error)
	Export(name string) io.ReadCloser
	Import(name string, reader io.Reader) error
	Remove(name string) error
}
type store struct {
	configFile     string
	currentContext string
	meta           *metadataStore
	tls            *tlsStore
}

// NewStore creates a store from a given directory.
// If the directory does not exist or is empty, initialize it
func NewStore(dir string) (Store, error) {
	metaRoot := filepath.Join(dir, metadataDir)
	tlsRoot := filepath.Join(dir, tlsDir)
	configFile := filepath.Join(dir, configFileName)
	err := os.MkdirAll(metaRoot, 0755)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(tlsRoot, 0700)
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(configFile)

	switch {
	case os.IsNotExist(err):
		//create default file
		err = ioutil.WriteFile(configFile, []byte("{}"), 0644)
		if err != nil {
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
	err = json.Unmarshal(configBytes, &cfg)
	if err != nil {
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

func (s *store) GetCurrentContext() string {
	return s.currentContext
}

func (s *store) SetCurrentContext(name string) error {
	cfg := config{CurrentContext: name}
	configBytes, err := json.Marshal(&cfg)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(s.configFile, configBytes, 0644)
	if err != nil {
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

func (s *store) GetContextMetadata(name string) (ContextMetadata, error) {
	return s.meta.get(name)
}

func (s *store) ResetContextTLSMaterial(name string, data *ContextTLSData) error {
	err := s.tls.removeAllContextData(name)
	if err != nil {
		return err
	}
	if data != nil {
		for ep, files := range data.Endpoints {
			for fileName, data := range files.Files {
				err = s.tls.createOrUpdate(name, ep, fileName, data)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *store) ResetContextEndpointTLSMaterial(contextName string, endpointName string, data *EndpointTLSData) error {
	err := s.tls.removeAllEndpointData(contextName, endpointName)
	if err != nil {
		return err
	}
	if data != nil {
		for fileName, data := range data.Files {
			err = s.tls.createOrUpdate(contextName, endpointName, fileName, data)
			if err != nil {
				return err
			}
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

func (s *store) Export(name string) io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		tw := tar.NewWriter(writer)
		defer tw.Flush()
		defer tw.Close()
		defer writer.Close()
		meta, err := s.meta.get(name)
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
		if err = appendDirToArchive(s.tls.contextDir(name), "tls/", tw); err != nil {
			writer.CloseWithError(err)
			return
		}
	}()
	return reader
}

func (s *store) Import(name string, reader io.Reader) error {
	tr := tar.NewReader(reader)
	metaDir := s.meta.contextDir(name)
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return err
	}
	tlsDir := s.tls.contextDir(name)
	if err := os.MkdirAll(tlsDir, 0700); err != nil {
		return err
	}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if hdr.Name == metaFile {
			data, err := ioutil.ReadAll(tr)
			if err != nil {
				return err
			}
			if err = ioutil.WriteFile(filepath.Join(metaDir, metaFile), data, 0644); err != nil {
				return err
			}
		} else if strings.HasPrefix(hdr.Name, "tls/") {
			relative := strings.TrimPrefix(hdr.Name, "tls/")
			path := filepath.Join(tlsDir, relative)
			dir := filepath.Dir(path)
			if err = os.MkdirAll(dir, 0700); err != nil {
				return err
			}
			data, err := ioutil.ReadAll(tr)
			if err != nil {
				return err
			}
			if err = ioutil.WriteFile(path, data, 0600); err != nil {
				return err
			}
		}
	}
}

func (s *store) Remove(name string) error {
	if err := s.meta.remove(name); err != nil {
		return err
	}
	return s.tls.removeAllContextData(name)
}

func appendDirToArchive(path, inArchivePrefix string, tw *tar.Writer) error {
	entries, err := ioutil.ReadDir(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if err = appendDirToArchive(filepath.Join(path, entry.Name()), inArchivePrefix+entry.Name()+"/", tw); err != nil {
				return err
			}
		} else {
			if err = tw.WriteHeader(&tar.Header{
				Mode: 0600,
				Name: inArchivePrefix + entry.Name(),
				Size: entry.Size(),
			}); err != nil {
				return err
			}
			bytes, err := ioutil.ReadFile(filepath.Join(path, entry.Name()))
			if err != nil {
				return err
			}
			if _, err = tw.Write(bytes); err != nil {
				return err
			}
		}
	}
	return nil
}

type config struct {
	CurrentContext string `json:"current_context,omitempty"`
}

// EndpointTLSData represents tls data for a given endpoint
type EndpointTLSData struct {
	Files map[string][]byte
}

// ContextTLSData represents tls data for a whole context
type ContextTLSData struct {
	Endpoints map[string]EndpointTLSData
}
