// Package avro provides th log driver for forwarding server logs to
// flume endpoints.
package avro

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/sebglon/goavro"
	"github.com/sebglon/goavro/transceiver"
	"github.com/sebglon/goavro/transceiver/netty"
	"net"
	"strconv"
)

type avro struct {
	hostname  string
	extra     map[string]interface{}
	conn      *net.TCPConn
	requestor *goavro.Requestor
	proto     goavro.Protocol
}

const (
	name = "flume-avro"

	defaultHost = "localhost"
	defaultPort = "63001"

	hostKey = "avro-host"
	portKey = "avro-port"
)

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New create a avro logger using the configuration passed in on
// the context.
func New(ctx logger.Context) (logger.Logger, error) {
	host := ctx.Config[hostKey]
	port := ctx.Config[portKey]

	logrus.Info("Avro logger socket: " + host + ":" + port)

	// collect extra data for Avro message
	hostname, err := ctx.Hostname()
	if err != nil {
		return nil, fmt.Errorf("Avro: cannot access hostname to set source field")
	}

	extra := map[string]interface{}{
		"host":              hostname,
		"daemon_name":       ctx.DaemonName,
		"container_id":      ctx.ContainerID,
		"container_name":    ctx.ContainerName,
		"image_id":          ctx.ContainerImageID,
		"image_name":        ctx.ContainerImageName,
		"command":           ctx.Command(),
		"container_created": strconv.FormatInt(ctx.ContainerCreated.Unix(), 10),
	}

	for k, v := range ctx.ContainerLabels {
		extra["container_label_"+k] = v
	}
	for k, v := range ctx.ExtraAttributes(nil) {
		extra["container_meta_"+k] = v
	}

	proto, err := goavro.NewProtocol()
	if err != nil {
		return nil, err
	}

	iport, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}

	transceiver, err := netty.NewTransceiver(transceiver.Config{Host: host, Port: iport})
	if err != nil {
		return nil, err
	}

	requestor := goavro.NewRequestor(proto, transceiver)

	logrus.Info("End init avro plugin")
	return &avro{
		extra:     extra,
		hostname:  hostname,
		requestor: requestor,
		proto:     proto,
	}, nil
}

func (a *avro) Log(msg *logger.Message) error {
	flumeRecord, errFlume := a.proto.NewRecord("AvroFlumeEvent")
	if errFlume != nil {
		return errFlume
	}
	headers := make(map[string]interface{})
	headers["source"] = msg.Source
	headers["partial"] = strconv.FormatBool(msg.Partial)
	headers["timestamp"] = msg.Timestamp.String()
	for k, v := range a.extra {
		headers[k] = v
	}
	flumeRecord.Set("headers", headers)
	flumeRecord.Set("body", []byte(msg.Line))

	logrus.WithFields(logrus.Fields{"flumeRecord": flumeRecord}).Info("Avro logger request")
	err := a.requestor.Request("append", flumeRecord)
	return err
}

func (a *avro) Close() error {
	return a.conn.Close()
}

func (a *avro) Name() string {
	return name
}

// ValidateLogOpt looks for avro specific log option avro-host avro-port.
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "env":
		case "labels":
		case hostKey:
		case portKey:
		//  Accepted
		default:
			return fmt.Errorf("unknown log opt '%s' for avro log driver", key)
		}
	}
	if len(cfg[hostKey]) == 0 {
		cfg[hostKey] = defaultHost
	}
	if len(cfg[portKey]) == 0 {
		cfg[portKey] = defaultPort
	}
	return nil
}
