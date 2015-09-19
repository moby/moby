// +build linux

// Package syslog provides the logdriver for forwarding server logs to syslog endpoints.
package syslog

import (
	"errors"
	"fmt"
	"log/syslog"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/docker/docker/pkg/urlutil"
)

const name = "syslog"

var facilities = map[string]syslog.Priority{
	"kern":     syslog.LOG_KERN,
	"user":     syslog.LOG_USER,
	"mail":     syslog.LOG_MAIL,
	"daemon":   syslog.LOG_DAEMON,
	"auth":     syslog.LOG_AUTH,
	"syslog":   syslog.LOG_SYSLOG,
	"lpr":      syslog.LOG_LPR,
	"news":     syslog.LOG_NEWS,
	"uucp":     syslog.LOG_UUCP,
	"cron":     syslog.LOG_CRON,
	"authpriv": syslog.LOG_AUTHPRIV,
	"ftp":      syslog.LOG_FTP,
	"local0":   syslog.LOG_LOCAL0,
	"local1":   syslog.LOG_LOCAL1,
	"local2":   syslog.LOG_LOCAL2,
	"local3":   syslog.LOG_LOCAL3,
	"local4":   syslog.LOG_LOCAL4,
	"local5":   syslog.LOG_LOCAL5,
	"local6":   syslog.LOG_LOCAL6,
	"local7":   syslog.LOG_LOCAL7,
}

type syslogger struct {
	writer *syslog.Writer
}

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New creates a syslog logger using the configuration passed in on
// the context. Supported context configuration variables are
// syslog-address, syslog-facility, & syslog-tag.
func New(ctx logger.Context) (logger.Logger, error) {
	tag, err := loggerutils.ParseLogTag(ctx, "{{.ID}}")
	if err != nil {
		return nil, err
	}

	proto, address, err := parseAddress(ctx.Config["syslog-address"])
	if err != nil {
		return nil, err
	}

	facility, err := parseFacility(ctx.Config["syslog-facility"])
	if err != nil {
		return nil, err
	}

	log, err := syslog.Dial(
		proto,
		address,
		facility,
		path.Base(os.Args[0])+"/"+tag,
	)
	if err != nil {
		return nil, err
	}

	return &syslogger{
		writer: log,
	}, nil
}

func (s *syslogger) Log(msg *logger.Message) error {
	if msg.Source == "stderr" {
		return s.writer.Err(string(msg.Line))
	}
	return s.writer.Info(string(msg.Line))
}

func (s *syslogger) Close() error {
	return s.writer.Close()
}

func (s *syslogger) Name() string {
	return name
}

func parseAddress(address string) (string, string, error) {
	if address == "" {
		return "", "", nil
	}
	if !urlutil.IsTransportURL(address) {
		return "", "", fmt.Errorf("syslog-address should be in form proto://address, got %v", address)
	}
	url, err := url.Parse(address)
	if err != nil {
		return "", "", err
	}

	// unix socket validation
	if url.Scheme == "unix" {
		if _, err := os.Stat(url.Path); err != nil {
			return "", "", err
		}
		return url.Scheme, url.Path, nil
	}

	// here we process tcp|udp
	host := url.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		if !strings.Contains(err.Error(), "missing port in address") {
			return "", "", err
		}
		host = host + ":514"
	}

	return url.Scheme, host, nil
}

// ValidateLogOpt looks for syslog specific log options
// syslog-address, syslog-facility, & syslog-tag.
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "syslog-address":
		case "syslog-facility":
		case "syslog-tag":
		case "tag":
		default:
			return fmt.Errorf("unknown log opt '%s' for syslog log driver", key)
		}
	}
	if _, _, err := parseAddress(cfg["syslog-address"]); err != nil {
		return err
	}
	if _, err := parseFacility(cfg["syslog-facility"]); err != nil {
		return err
	}
	return nil
}

func parseFacility(facility string) (syslog.Priority, error) {
	if facility == "" {
		return syslog.LOG_DAEMON, nil
	}

	if syslogFacility, valid := facilities[facility]; valid {
		return syslogFacility, nil
	}

	fInt, err := strconv.Atoi(facility)
	if err == nil && 0 <= fInt && fInt <= 23 {
		return syslog.Priority(fInt << 3), nil
	}

	return syslog.Priority(0), errors.New("invalid syslog facility")
}
