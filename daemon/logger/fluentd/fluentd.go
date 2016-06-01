// Package fluentd provides the log driver for forwarding server logs
// to fluentd endpoints.
package fluentd

import (
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/docker/go-units"
	"github.com/fluent/fluent-logger-golang/fluent"
)

type fluentd struct {
	tag           string
	containerID   string
	containerName string
	writer        *fluent.Fluent
	extra         map[string]string
}

const (
	name = "fluentd"

	defaultHost        = "127.0.0.1"
	defaultPort        = 24224
	defaultBufferLimit = 1024 * 1024
	defaultTagPrefix   = "docker"

	// logger tries to reconnect 2**32 - 1 times
	// failed (and panic) after 204 years [ 1.5 ** (2**32 - 1) - 1 seconds]
	defaultRetryWait              = 1000
	defaultTimeout                = 3 * time.Second
	defaultMaxRetries             = math.MaxInt32
	defaultReconnectWaitIncreRate = 1.5

	addressKey      = "fluentd-address"
	bufferLimitKey  = "fluentd-buffer-limit"
	retryWaitKey    = "fluentd-retry-wait"
	maxRetriesKey   = "fluentd-max-retries"
	asyncConnectKey = "fluentd-async-connect"
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
// the context. The supported context configuration variable is
// fluentd-address.
func New(ctx logger.Context) (logger.Logger, error) {
	host, port, err := parseAddress(ctx.Config[addressKey])
	if err != nil {
		return nil, err
	}

	tag, err := loggerutils.ParseLogTag(ctx, "{{.DaemonName}}.{{.ID}}")
	if err != nil {
		return nil, err
	}

	extra := ctx.ExtraAttributes(nil)

	bufferLimit := defaultBufferLimit
	if ctx.Config[bufferLimitKey] != "" {
		bl64, err := units.RAMInBytes(ctx.Config[bufferLimitKey])
		if err != nil {
			return nil, err
		}
		bufferLimit = int(bl64)
	}

	retryWait := defaultRetryWait
	if ctx.Config[retryWaitKey] != "" {
		rwd, err := time.ParseDuration(ctx.Config[retryWaitKey])
		if err != nil {
			return nil, err
		}
		retryWait = int(rwd.Seconds() * 1000)
	}

	maxRetries := defaultMaxRetries
	if ctx.Config[maxRetriesKey] != "" {
		mr64, err := strconv.ParseUint(ctx.Config[maxRetriesKey], 10, strconv.IntSize)
		if err != nil {
			return nil, err
		}
		maxRetries = int(mr64)
	}

	asyncConnect := false
	if ctx.Config[asyncConnectKey] != "" {
		if asyncConnect, err = strconv.ParseBool(ctx.Config[asyncConnectKey]); err != nil {
			return nil, err
		}
	}

	fluentConfig := fluent.Config{
		FluentPort:   port,
		FluentHost:   host,
		BufferLimit:  bufferLimit,
		RetryWait:    retryWait,
		MaxRetry:     maxRetries,
		AsyncConnect: asyncConnect,
	}

	logrus.WithField("container", ctx.ContainerID).WithField("config", fluentConfig).
		Debug("logging driver fluentd configured")

	log, err := fluent.New(fluentConfig)
	if err != nil {
		return nil, err
	}
	return &fluentd{
		tag:           tag,
		containerID:   ctx.ContainerID,
		containerName: ctx.ContainerName,
		writer:        log,
		extra:         extra,
	}, nil
}

func (f *fluentd) Log(msg *logger.Message) error {
	data := map[string]string{
		"container_id":   f.containerID,
		"container_name": f.containerName,
		"source":         msg.Source,
		"log":            string(msg.Line),
	}
	for k, v := range f.extra {
		data[k] = v
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

// ValidateLogOpt looks for fluentd specific log option fluentd-address.
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "env":
		case "labels":
		case "tag":
		case addressKey:
		case bufferLimitKey:
		case retryWaitKey:
		case maxRetriesKey:
		case asyncConnectKey:
			// Accepted
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
		return defaultHost, defaultPort, nil
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
