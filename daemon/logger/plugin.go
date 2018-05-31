package logger // import "github.com/docker/docker/daemon/logger"

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/plugins/logdriver"
	"github.com/docker/docker/errdefs"
	getter "github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
)

var pluginGetter getter.PluginGetter

const extName = "LogDriver"

// logPlugin defines the available functions that logging plugins must implement.
type logPlugin interface {
	StartLogging(streamPath string, info Info) (err error)
	StopLogging(streamPath string) (err error)
	Capabilities() (cap Capability, err error)
	ReadLogs(info Info, config ReadConfig) (stream io.ReadCloser, err error)
}

// RegisterPluginGetter sets the plugingetter
func RegisterPluginGetter(plugingetter getter.PluginGetter) {
	pluginGetter = plugingetter
}

// GetDriver returns a logging driver by its name.
// If the driver is empty, it looks for the local driver.
func getPlugin(name string, mode int) (Creator, error) {
	p, err := pluginGetter.Get(name, extName, mode)
	if err != nil {
		return nil, fmt.Errorf("error looking up logging plugin %s: %v", name, err)
	}

	client, err := makePluginClient(p)
	if err != nil {
		return nil, err
	}
	return makePluginCreator(name, client, p.ScopedPath), nil
}

func makePluginClient(p getter.CompatPlugin) (logPlugin, error) {
	if pc, ok := p.(getter.PluginWithV1Client); ok {
		return &logPluginProxy{pc.Client()}, nil
	}
	pa, ok := p.(getter.PluginAddr)
	if !ok {
		return nil, errdefs.System(errors.Errorf("got unknown plugin type %T", p))
	}

	if pa.Protocol() != plugins.ProtocolSchemeHTTPV1 {
		return nil, errors.Errorf("plugin protocol not supported: %s", p)
	}

	addr := pa.Addr()
	c, err := plugins.NewClientWithTimeout(addr.Network()+"://"+addr.String(), nil, pa.Timeout())
	if err != nil {
		return nil, errors.Wrap(err, "error making plugin client")
	}
	return &logPluginProxy{c}, nil
}

func makePluginCreator(name string, l logPlugin, scopePath func(s string) string) Creator {
	return func(logCtx Info) (logger Logger, err error) {
		defer func() {
			if err != nil {
				pluginGetter.Get(name, extName, getter.Release)
			}
		}()

		unscopedPath := filepath.Join("/", "run", "docker", "logging")
		logRoot := scopePath(unscopedPath)
		if err := os.MkdirAll(logRoot, 0700); err != nil {
			return nil, err
		}

		id := stringid.GenerateNonCryptoID()
		a := &pluginAdapter{
			driverName: name,
			id:         id,
			plugin:     l,
			fifoPath:   filepath.Join(logRoot, id),
			logInfo:    logCtx,
		}

		cap, err := a.plugin.Capabilities()
		if err == nil {
			a.capabilities = cap
		}

		stream, err := openPluginStream(a)
		if err != nil {
			return nil, err
		}

		a.stream = stream
		a.enc = logdriver.NewLogEntryEncoder(a.stream)

		if err := l.StartLogging(filepath.Join(unscopedPath, id), logCtx); err != nil {
			return nil, errors.Wrapf(err, "error creating logger")
		}

		if cap.ReadLogs {
			return &pluginAdapterWithRead{a}, nil
		}

		return a, nil
	}
}
