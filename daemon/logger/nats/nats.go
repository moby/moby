// Package nats provides a logging driver for emitting logs
// to NATS in JSON format.
package nats

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/nats-io/nats"
)

const name = "nats"

var secureOpts = []string{"nats-tls-ca-cert", "nats-tls-cert", "nats-tls-key", "nats-tls-skip-verify"}

// natsLogger contains the captured values from the context
// at the moment that the container was started.
type natsLogger struct {
	nc      *nats.Conn
	c       *nats.EncodedConn
	fields  map[string]interface{}
	subject string
}

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New creates a nats connection using the custom configuration values
// and container metadata passed in on the context
func New(ctx logger.Context) (logger.Logger, error) {
	opts := nats.DefaultOptions

	// e.g --log-opt nats-servers="nats://127.0.0.1:4222,nats://127.0.0.1:4223"
	if v, ok := ctx.Config["nats-servers"]; ok {
		opts.Servers = processURLString(v)
	} else {
		opts.Servers = []string{nats.DefaultURL}
	}

	// --log-opt nats-max-reconnect=-1
	if v, ok := ctx.Config["nats-max-reconnect"]; ok {
		i, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}

		opts.MaxReconnect = i
	} else {
		// Never stop reconnecting by default
		opts.MaxReconnect = -1
	}

	// Specify authentication credentials if required
	if v, ok := ctx.Config["nats-user"]; ok {
		opts.User = v
	}
	if v, ok := ctx.Config["nats-pass"]; ok {
		opts.Password = v
	}
	if v, ok := ctx.Config["nats-token"]; ok {
		opts.Token = v
	}

	// Check whether we need customize a secure connection with TLS
	var requiresTLS bool
	for _, sopt := range secureOpts {
		if _, ok := ctx.Config[sopt]; ok {
			requiresTLS = true
			break
		}
	}
	if requiresTLS {
		_, skipVerify := ctx.Config["nats-tls-skip-verify"]

		tlsOpts := tlsconfig.Options{
			CAFile:             ctx.Config["nats-tls-ca-cert"],
			CertFile:           ctx.Config["nats-tls-cert"],
			KeyFile:            ctx.Config["nats-tls-key"],
			InsecureSkipVerify: skipVerify,
		}
		tlsConfig, err := tlsconfig.Client(tlsOpts)
		if err != nil {
			return nil, err
		}
		opts.Secure = true
		opts.TLSConfig = tlsConfig
	}

	// Use standardized tag for the events and default subject
	tag, err := loggerutils.ParseLogTag(ctx, loggerutils.DefaultTemplate)
	if err != nil {
		return nil, err
	}

	// Subject under which log entries will be published, defaults
	// to using tag as the subject name.
	var subject string
	if v, ok := ctx.Config["nats-subject"]; ok {
		subject = v
	} else {
		subject = tag
	}

	// Use container image name to label client connection
	opts.Name = ctx.ContainerImageName

	// Create a single connection per container
	nc, err := opts.Connect()
	if err != nil {
		return nil, err
	}

	c, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		return nil, err
	}

	logrus.WithField("container", ctx.ContainerID).Infof("nats: connected to %q", nc.ConnectedUrl())

	// Set handlers to log in case of events related to the established connection
	nc.SetDisconnectHandler(func(c *nats.Conn) {
		logrus.WithField("container", ctx.ContainerID).Warnf("nats: disconnected")
	})

	nc.SetReconnectHandler(func(c *nats.Conn) {
		logrus.WithField("container", ctx.ContainerID).Warnf("nats: reconnected to %q", c.ConnectedUrl())
	})

	nc.SetClosedHandler(func(c *nats.Conn) {
		logrus.WithField("container", ctx.ContainerID).Warnf("nats: connection closed")
	})

	// Include hostname info in the record message
	hostname, err := ctx.Hostname()
	if err != nil {
		return nil, err
	}

	// Remove trailing slash from container name
	containerName := bytes.TrimLeft([]byte(ctx.ContainerName), "/")

	fields := make(map[string]interface{})
	fields["container_id"] = ctx.ContainerID
	fields["container_name"] = string(containerName)
	fields["image_id"] = ctx.ContainerImageID
	fields["image_name"] = ctx.ContainerImageName
	fields["hostname"] = hostname
	fields["tag"] = tag

	extra := ctx.ExtraAttributes(nil)
	for k, v := range extra {
		fields[k] = v
	}

	return &natsLogger{
		nc:      nc,
		c:       c,
		subject: subject,
		fields:  fields,
	}, nil
}

// ValidateLogOpt looks for nats logger related custom options.
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "env":
		case "labels":
		case "nats-max-reconnect":
		case "nats-servers":
		case "nats-subject":
		case "nats-user":
		case "nats-pass":
		case "nats-token":
		case "nats-tls-ca-cert":
		case "nats-tls-cert":
		case "nats-tls-key":
		case "nats-tls-skip-verify":
		case "tag":
		default:
			return fmt.Errorf("unknown log opt %q for nats log driver", key)
		}
	}

	return nil
}

// Log takes a message and puts it on the flushing queue from
// the NATS client.
func (nl *natsLogger) Log(msg *logger.Message) error {
	fields := make(map[string]interface{})
	for k, v := range nl.fields {
		fields[k] = v
	}
	fields["time"] = msg.Timestamp.UTC()
	fields["text"] = string(msg.Line)
	fields["source"] = msg.Source

	return nl.c.Publish(nl.subject, fields)
}

// Close gracefully terminates the connection to NATS,
// flushing any pending events before disconnecting.
func (nl *natsLogger) Close() error {
	nl.c.Close()
	return nil
}

// Name returns the name label from the NATS logger.
func (nl *natsLogger) Name() string {
	return name
}

// Process the url string argument to Connect. Return an array of
// urls, even if only one.
func processURLString(url string) []string {
	urls := strings.Split(url, ",")
	for i, s := range urls {
		urls[i] = strings.TrimSpace(s)
	}
	return urls
}
