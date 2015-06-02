// +build linux

package syslog

import (
	"io"
	"log/syslog"
	"net"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/urlutil"
)

const name = "syslog"

type Syslog struct {
	writer *syslog.Writer
}

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
}

func New(ctx logger.Context) (logger.Logger, error) {
	tag := ctx.Config["syslog-tag"]
	if tag == "" {
		tag = ctx.ContainerID[:12]
	}

	proto, address, err := parseAddress(ctx.Config["syslog-address"])
	if err != nil {
		return nil, err
	}

	log, err := syslog.Dial(
		proto,
		address,
		syslog.LOG_DAEMON,
		path.Base(os.Args[0])+"/"+tag,
	)
	if err != nil {
		return nil, err
	}

	return &Syslog{
		writer: log,
	}, nil
}

func (s *Syslog) Log(msg *logger.Message) error {
	if msg.Source == "stderr" {
		return s.writer.Err(string(msg.Line))
	}
	return s.writer.Info(string(msg.Line))
}

func (s *Syslog) Close() error {
	return s.writer.Close()
}

func (s *Syslog) Name() string {
	return name
}

func (s *Syslog) GetReader() (io.Reader, error) {
	return nil, logger.ReadLogsNotSupported
}

func parseAddress(address string) (string, string, error) {
	if urlutil.IsTransportURL(address) {
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

	return "", "", nil
}
