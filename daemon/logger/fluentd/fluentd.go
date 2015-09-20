// Package fluentd provides the log driver for forwarding server logs
// to fluentd endpoints.
package fluentd

import (
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/fluent/fluent-logger-golang/fluent"
)

type fluentd struct {
	tag           string
	containerID   string
	containerName string
	writer        *fluent.Fluent
}

const (
	name             = "fluentd"
	defaultHostName  = "localhost"
	defaultPort      = 24224
	defaultTagPrefix = "docker"
)

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New creates a fluentd logger using the configuration passed in on
// the context. Supported context configuration variables are
// fluentd-address & fluentd-tag.
func New(ctx logger.Context) (logger.Logger, error) {
	host, port, err := parseAddress(ctx.Config["fluentd-address"])
	if err != nil {
		return nil, err
	}

	tag, err := loggerutils.ParseLogTag(ctx, "docker.{{.ID}}")
	if err != nil {
		return nil, err
	}

	logrus.Debugf("logging driver fluentd configured for container:%s, host:%s, port:%d, tag:%s.", ctx.ContainerID, host, port, tag)

	// logger tries to recoonect 2**32 - 1 times
	// failed (and panic) after 204 years [ 1.5 ** (2**32 - 1) - 1 seconds]
	log, err := fluent.New(fluent.Config{FluentPort: port, FluentHost: host, RetryWait: 1000, MaxRetry: math.MaxInt32})
	if err != nil {
		return nil, err
	}
	return &fluentd{
		tag:           tag,
		containerID:   ctx.ContainerID,
		containerName: ctx.ContainerName,
		writer:        log,
	}, nil
}

func (f *fluentd) Log(msg *logger.Message) error {
	data := map[string]string{
		"container_id":   f.containerID,
		"container_name": f.containerName,
		"source":         msg.Source,
		"log":            string(msg.Line),
	}
	// fluent-logger-golang buffers logs from failures and disconnections,
	// and these are transferred again automatically.
	return f.writer.PostWithTime(f.tag, msg.Timestamp, data)
}

func (f *fluentd) Close() error {
	return f.writer.Close()
}

func (f *fluentd) Name() string {
	return name
}

// ValidateLogOpt looks for fluentd specific log options fluentd-address & fluentd-tag.
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "fluentd-address":
		case "fluentd-tag":
		case "tag":
		default:
			return fmt.Errorf("unknown log opt '%s' for fluentd log driver", key)
		}
	}

	if _, _, err := parseAddress(cfg["fluentd-address"]); err != nil {
		return err
	}

	return nil
}

func parseAddress(address string) (string, int, error) {
	if address == "" {
		return defaultHostName, defaultPort, nil
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		if !strings.Contains(err.Error(), "missing port in address") {
			return "", 0, fmt.Errorf("invalid fluentd-address %s: %s", address, err)
		}
		return host, defaultPort, nil
	}

	portnum, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, fmt.Errorf("invalid fluentd-address %s: %s", address, err)
	}
	return host, portnum, nil
}
