package container

import (
	"context"
	"io"
	"maps"
	"slices"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/logger"
	"github.com/moby/moby/v2/daemon/server/backend"
)

// startLogCapture tees the exec's stdout and stderr into the container's
// logging driver, stamping each message with the exec's identity so log
// readers can attribute the output. It is called from InitializeStdio when
// the exec was created with CaptureLogs.
func (c *ExecConfig) startLogCapture() {
	c.Container.Lock()
	logDriver := c.Container.LogDriver
	c.Container.Unlock()
	if logDriver == nil {
		// Checked at exec create time; the driver can still be missing if
		// the container is being torn down — nothing to capture into.
		log.G(context.TODO()).WithField("exec", c.ID).Warn("no logging driver available, exec output will not be captured")
		return
	}

	attrs := []backend.LogAttr{{Key: "exec_id", Value: c.ID}}
	for _, k := range slices.Sorted(maps.Keys(c.Labels)) {
		attrs = append(attrs, backend.LogAttr{Key: k, Value: c.Labels[k]})
	}

	// The driver is shared with the container's own stdio copier; drivers
	// already accept concurrent Log calls from the stdout and stderr copy
	// goroutines, two more sources follow the same contract.
	copier := logger.NewCopier(map[string]io.Reader{
		"stdout": c.StreamConfig.StdoutPipe(),
		"stderr": c.StreamConfig.StderrPipe(),
	}, &execLogger{Logger: logDriver, attrs: attrs})
	c.LogCopier = copier
	copier.Run()
}

// execLogger stamps the exec's identity on each captured message before
// handing it to the container's logging driver.
type execLogger struct {
	logger.Logger
	attrs []backend.LogAttr
}

// Log implements logger.Logger, decorating each message with the exec's
// attributes.
func (l *execLogger) Log(msg *logger.Message) error {
	msg.Attrs = append(msg.Attrs, l.attrs...)
	return l.Logger.Log(msg)
}
