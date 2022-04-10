// Package fluentd provides the log driver for forwarding server logs
// to fluentd endpoints.
package fluentd // import "github.com/docker/docker/daemon/logger/fluentd"

import (
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/urlutil"
	units "github.com/docker/go-units"
	"github.com/fluent/fluent-logger-golang/fluent"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type fluentd struct {
	tag           string
	containerID   string
	containerName string
	writer        *fluent.Fluent
	extra         map[string]string
}

type location struct {
	protocol string
	host     string
	port     int
	path     string
}

const (
	name = "fluentd"

	defaultBufferLimit = 1024 * 1024
	defaultHost        = "127.0.0.1"
	defaultPort        = 24224
	defaultProtocol    = "tcp"

	// logger tries to reconnect 2**32 - 1 times
	// failed (and panic) after 204 years [ 1.5 ** (2**32 - 1) - 1 seconds]
	defaultMaxRetries = math.MaxInt32
	defaultRetryWait  = 1000

	minReconnectInterval = 100 * time.Millisecond
	maxReconnectInterval = 10 * time.Second

	addressKey                = "fluentd-address"
	asyncKey                  = "fluentd-async"
	asyncConnectKey           = "fluentd-async-connect" // deprecated option (use fluent-async instead)
	asyncReconnectIntervalKey = "fluentd-async-reconnect-interval"
	bufferLimitKey            = "fluentd-buffer-limit"
	maxRetriesKey             = "fluentd-max-retries"
	requestAckKey             = "fluentd-request-ack"
	retryWaitKey              = "fluentd-retry-wait"
	subSecondPrecisionKey     = "fluentd-sub-second-precision"
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
func New(info logger.Info) (logger.Logger, error) {
	fluentConfig, err := parseConfig(info.Config)
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	tag, err := loggerutils.ParseLogTag(info, loggerutils.DefaultTemplate)
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	extra, err := info.ExtraAttributes(nil)
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	logrus.WithField("container", info.ContainerID).WithField("config", fluentConfig).
		Debug("logging driver fluentd configured")

	log, err := fluent.New(fluentConfig)
	if err != nil {
		return nil, err
	}
	return &fluentd{
		tag:           tag,
		containerID:   info.ContainerID,
		containerName: info.ContainerName,
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
	if msg.PLogMetaData != nil {
		data["partial_message"] = "true"
		data["partial_id"] = msg.PLogMetaData.ID
		data["partial_ordinal"] = strconv.Itoa(msg.PLogMetaData.Ordinal)
		data["partial_last"] = strconv.FormatBool(msg.PLogMetaData.Last)
	}

	ts := msg.Timestamp
	logger.PutMessage(msg)
	// fluent-logger-golang buffers logs from failures and disconnections,
	// and these are transferred again automatically.
	return f.writer.PostWithTime(f.tag, ts, data)
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
		case "env-regex":
		case "labels":
		case "labels-regex":
		case "tag":

		case addressKey:
		case asyncKey:
		case asyncConnectKey:
		case asyncReconnectIntervalKey:
		case bufferLimitKey:
		case maxRetriesKey:
		case requestAckKey:
		case retryWaitKey:
		case subSecondPrecisionKey:
			// Accepted
		default:
			return errors.Errorf("unknown log opt '%s' for fluentd log driver", key)
		}
	}

	_, err := parseConfig(cfg)
	return err
}

func parseConfig(cfg map[string]string) (fluent.Config, error) {
	var config fluent.Config

	loc, err := parseAddress(cfg[addressKey])
	if err != nil {
		return config, err
	}

	bufferLimit := defaultBufferLimit
	if cfg[bufferLimitKey] != "" {
		bl64, err := units.RAMInBytes(cfg[bufferLimitKey])
		if err != nil {
			return config, err
		}
		bufferLimit = int(bl64)
	}

	retryWait := defaultRetryWait
	if cfg[retryWaitKey] != "" {
		rwd, err := time.ParseDuration(cfg[retryWaitKey])
		if err != nil {
			return config, err
		}
		retryWait = int(rwd.Seconds() * 1000)
	}

	maxRetries := defaultMaxRetries
	if cfg[maxRetriesKey] != "" {
		mr64, err := strconv.ParseUint(cfg[maxRetriesKey], 10, strconv.IntSize)
		if err != nil {
			return config, err
		}
		maxRetries = int(mr64)
	}

	if cfg[asyncKey] != "" && cfg[asyncConnectKey] != "" {
		return config, errors.Errorf("conflicting options: cannot specify both '%s' and '%s", asyncKey, asyncConnectKey)
	}

	async := false
	if cfg[asyncKey] != "" {
		if async, err = strconv.ParseBool(cfg[asyncKey]); err != nil {
			return config, err
		}
	}

	// TODO fluentd-async-connect is deprecated in driver v1.4.0. Remove after two stable releases
	asyncConnect := false
	if cfg[asyncConnectKey] != "" {
		if asyncConnect, err = strconv.ParseBool(cfg[asyncConnectKey]); err != nil {
			return config, err
		}
	}

	asyncReconnectInterval := 0
	if cfg[asyncReconnectIntervalKey] != "" {
		interval, err := time.ParseDuration(cfg[asyncReconnectIntervalKey])
		if err != nil {
			return config, errors.Wrapf(err, "invalid value for %s", asyncReconnectIntervalKey)
		}
		if interval != 0 && (interval < minReconnectInterval || interval > maxReconnectInterval) {
			return config, errors.Errorf("invalid value for %s: value (%q) must be between %s and %s",
				asyncReconnectIntervalKey, interval, minReconnectInterval, maxReconnectInterval)
		}
		asyncReconnectInterval = int(interval.Milliseconds())
	}

	subSecondPrecision := false
	if cfg[subSecondPrecisionKey] != "" {
		if subSecondPrecision, err = strconv.ParseBool(cfg[subSecondPrecisionKey]); err != nil {
			return config, err
		}
	}

	requestAck := false
	if cfg[requestAckKey] != "" {
		if requestAck, err = strconv.ParseBool(cfg[requestAckKey]); err != nil {
			return config, err
		}
	}

	config = fluent.Config{
		FluentPort:             loc.port,
		FluentHost:             loc.host,
		FluentNetwork:          loc.protocol,
		FluentSocketPath:       loc.path,
		BufferLimit:            bufferLimit,
		RetryWait:              retryWait,
		MaxRetry:               maxRetries,
		Async:                  async,
		AsyncConnect:           asyncConnect,
		AsyncReconnectInterval: asyncReconnectInterval,
		SubSecondPrecision:     subSecondPrecision,
		RequestAck:             requestAck,
		ForceStopAsyncSend:     async || asyncConnect,
	}

	return config, nil
}

func parseAddress(address string) (*location, error) {
	if address == "" {
		return &location{
			protocol: defaultProtocol,
			host:     defaultHost,
			port:     defaultPort,
			path:     "",
		}, nil
	}

	protocol := defaultProtocol
	givenAddress := address
	if urlutil.IsTransportURL(address) {
		addr, err := url.Parse(address)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid fluentd-address %s", givenAddress)
		}
		// unix and unixgram socket
		if addr.Scheme == "unix" || addr.Scheme == "unixgram" {
			return &location{
				protocol: addr.Scheme,
				host:     "",
				port:     0,
				path:     addr.Path,
			}, nil
		}
		// tcp|udp
		protocol = addr.Scheme
		address = addr.Host
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		if !strings.Contains(err.Error(), "missing port in address") {
			return nil, errors.Wrapf(err, "invalid fluentd-address %s", givenAddress)
		}
		return &location{
			protocol: protocol,
			host:     host,
			port:     defaultPort,
			path:     "",
		}, nil
	}

	portnum, err := strconv.Atoi(port)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid fluentd-address %s", givenAddress)
	}
	return &location{
		protocol: protocol,
		host:     host,
		port:     portnum,
		path:     "",
	}, nil
}
