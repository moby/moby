package docker

import (
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/nat"
	"github.com/dotcloud/docker/pkg/namesgenerator"
	"github.com/dotcloud/docker/runconfig"
	"github.com/dotcloud/docker/utils"
	"io"
	"sync/atomic"
)

type Change struct {
	archive.Change
}

func migratePortMappings(config *runconfig.Config, hostConfig *runconfig.HostConfig) error {
	if config.PortSpecs != nil {
		ports, bindings, err := nat.ParsePortSpecs(config.PortSpecs)
		if err != nil {
			return err
		}
		config.PortSpecs = nil
		if len(bindings) > 0 {
			if hostConfig == nil {
				hostConfig = &runconfig.HostConfig{}
			}
			hostConfig.PortBindings = bindings
		}

		if config.ExposedPorts == nil {
			config.ExposedPorts = make(nat.PortSet, len(ports))
		}
		for k, v := range ports {
			config.ExposedPorts[k] = v
		}
	}
	return nil
}

// Links come in the format of
// name:alias
func parseLink(rawLink string) (map[string]string, error) {
	return utils.PartParser("name:alias", rawLink)
}

type checker struct {
	runtime *Runtime
}

func (c *checker) Exists(name string) bool {
	return c.runtime.containerGraph.Exists("/" + name)
}

// Generate a random and unique name
func generateRandomName(runtime *Runtime) (string, error) {
	return namesgenerator.GenerateRandomName(&checker{runtime})
}

// Read an io.Reader and call a function when it returns EOF
func EofReader(r io.Reader, callback func()) *eofReader {
	return &eofReader{
		Reader:   r,
		callback: callback,
	}
}

type eofReader struct {
	io.Reader
	gotEOF   int32
	callback func()
}

func (r *eofReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if err == io.EOF {
		// Use atomics to make the gotEOF check threadsafe
		if atomic.CompareAndSwapInt32(&r.gotEOF, 0, 1) {
			r.callback()
		}
	}
	return
}
