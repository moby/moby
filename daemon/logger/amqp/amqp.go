// +build linux

package amqp

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/streadway/amqp"
)

const name = "amqp"

type amqpLogger struct {
	ctx        logger.Context
	fields     amqpFields
	connection *amqpConnection
}

// Data structure holding information about the current connection
// with a broker as well as a list of other available brokers
type amqpConnection struct {
	broker     int
	brokerURLs []*amqpBroker
	conn       *amqp.Connection
	c          *amqp.Channel
	conf       <-chan amqp.Confirmation
	err        error
}

// Data structure to hold the connection settings for each broker
type amqpBroker struct {
	BrokerURL  string `json:"url"`
	CertPath   string `json:"cert"`
	KeyPath    string `json:"key"`
	Exchange   string `json:"exchange"`
	Queue      string `json:"queue"`
	RoutingKey string `json:"routingkey"`
	Tag        string `json:"tag"`
	Confirm    bool   `json:"confirm,bool"`
}

// Data structure to store the data for the log message
type amqpMessage struct {
	Message   string     `json:"message"`
	Version   string     `json:"@version"`
	Timestamp time.Time  `json:"@timestamp"`
	Tags      amqpFields `json:"tags"`
	Host      string     `json:"host"`
	Path      string     `json:"path"`
}

// Data about the host and container that is required when sending
// the log message
type amqpFields struct {
	Hostname      string
	ContainerID   string
	ContainerName string
	ImageID       string
	ImageName     string
	Command       string
	Tag           string
	AMQPTag       string
	Created       time.Time
}

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New creates a new amqp logger using the configuration passed in the
// context.
func New(ctx logger.Context) (logger.Logger, error) {
	// collect extra data for AMQP message
	hostname, err := ctx.Hostname()
	if err != nil {
		return nil, fmt.Errorf("Cannot access hostname to set source field: %v", err)
	}

	logrus.Infof("URLs: %v", ctx.Config["amqp-url"])

	// remove trailing slash from container name
	containerName := bytes.TrimLeft([]byte(ctx.ContainerName), "/")

	tag, err := loggerutils.ParseLogTag(ctx, "")
	if err != nil {
		return nil, err
	}

	fields := amqpFields{
		Hostname:      hostname,
		ContainerID:   ctx.ContainerID,
		ContainerName: string(containerName),
		ImageID:       ctx.ContainerImageID,
		ImageName:     ctx.ContainerImageName,
		Command:       ctx.Command(),
		Tag:           tag,
		AMQPTag:       ctx.Config["amqp-tag"],
		Created:       ctx.ContainerCreated,
	}

	connection, err := connect(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("Could not connect: %v", err)
	}

	return &amqpLogger{
		ctx:        ctx,
		fields:     fields,
		connection: connection,
	}, nil
}

// Connect to a broker using the information provided in the logger context. If there is more
// than one broker in a list then try to connect to the one identified by the broker integer.
// Return an amqpConnection.
func connect(ctx logger.Context, broker int) (connection *amqpConnection, err error) {
	var conn *amqp.Connection
	var c *amqp.Channel

	var conf <-chan amqp.Confirmation
	var brokerURLs []*amqpBroker
	// Check if a configuration file has been specified, if not then
	// take the settings from the log-driver options
	if ctx.Config["amqp-settings"] == "" {
		brokerURLs = parseURL(ctx)
	} else {
		brokerURLs = parseJSONFile(ctx.Config["amqp-settings"])
	}
	logrus.Info(brokerURLs)
	if err != nil {
		logrus.Errorf("Invalid AMQP URL - %v", err)
		return nil, err
	}

	currentBroker := brokerURLs[broker]
	brokerURL, err := url.Parse(currentBroker.BrokerURL)
	if err != nil {
		logrus.Errorf("Could not read URL from JSON file: %v", err)
		return nil, err
	}

	// If the broker is being connected to using TLS then find the certificate and key files.
	// If not then connect normally.
	if brokerURL.Scheme == "amqps" {
		logrus.Infof("Connecting to AMQP: %s", brokerURL)

		cfg := new(tls.Config)
		if cert, err := tls.LoadX509KeyPair(currentBroker.CertPath, currentBroker.KeyPath); err == nil {
			cfg.Certificates = append(cfg.Certificates, cert)
		}
		conn, err = amqp.DialTLS(brokerURL.String(), cfg)
		if err != nil {
			logrus.Errorf("Could not connect to AMQP server - %v", err)
			return nil, err
		}
	} else {
		logrus.Infof("Connecting to AMQP: %s", brokerURL)
		conn, err = amqp.Dial(brokerURL.String())
		if err != nil {
			logrus.Errorf("Could not connect to AMQP server - %v", err)
			return nil, err
		}
	}

	c, err = conn.Channel()
	if err != nil {
		logrus.Errorf("Could not open channel - %v", err)
		return nil, err
	}

	if currentBroker.Confirm == true {
		logrus.Info("Enabling publish confirmation")
		if err := c.Confirm(false); err != nil {
			logrus.Errorf("Could not put channel into confirm mode - %v", err)
			return nil, err
		}

		conf = c.NotifyPublish(make(chan amqp.Confirmation, 1))
	}

	err = c.ExchangeDeclare(currentBroker.Exchange, "direct", true, false, false, false, nil)
	if err != nil {
		logrus.Errorf("Could not create exchange - %v", err)
		return nil, err
	}

	_, err = c.QueueDeclare(currentBroker.Queue, true, false, false, false, nil)
	if err != nil {
		logrus.Errorf("Could not create queue - %v", err)
		return nil, err
	}

	err = c.QueueBind(currentBroker.Queue, currentBroker.RoutingKey, currentBroker.Exchange, false, nil)
	if err != nil {
		logrus.Errorf("Could not bind queue to exchange - %v", err)
		return nil, err
	}

	logrus.Info("Connection set up")
	return &amqpConnection{
		broker:     broker,
		brokerURLs: brokerURLs,
		conn:       conn,
		c:          c,
		conf:       conf,
		err:        err,
	}, nil
}

// If the connection fails at any point then close the current connection and
// try to connect to the next broker in the list.
func reconnect(s *amqpLogger) (err error) {
	logrus.Warn("Unable to send message to AMQP broker")
	logrus.Info("Attempting to reconnect")
	s.Close()
	// Move to the next broker in the list. If at the end of the
	// list then go back to the start
	if len(s.connection.brokerURLs) > s.connection.broker+1 {
		s.connection.broker++
	} else {
		s.connection.broker = 0
	}
	connection, err := connect(s.ctx, s.connection.broker)
	if err != nil {
		logrus.Errorf("Could not reconnect: %v", err)
		return err
	}
	logrus.Info("Reconnected")
	s.connection = connection
	return nil
}

// Take the log message and publish it to the currently connected broker
func (s *amqpLogger) Log(msg *logger.Message) (err error) {
	// Remove trailing and leading whitespace
	short := bytes.TrimSpace([]byte(msg.Line))

	currentBroker := s.connection.brokerURLs[s.connection.broker]

	// If the message isn't empty then send it to the broker
	if string(short) != "" {
		m := amqpMessage{
			Version:   "1",
			Host:      s.fields.Hostname,
			Message:   string(short),
			Timestamp: time.Now(),
			Path:      s.fields.ContainerID,
			Tags:      s.fields,
		}

		messagejson, err := json.Marshal(m)
		if err != nil {
			logrus.Errorf("Could not serialise event - %v", err)
			return err
		}

		amqpmsg := amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			ContentType:  "application/json",
			Body:         messagejson,
		}

		if currentBroker.Confirm == true && s.connection != nil {
			defer confirmOne(s.connection.conf)
		}

		err = s.connection.c.Publish(currentBroker.Exchange, currentBroker.RoutingKey, false, false, amqpmsg)
		if err != nil {
			err = reconnect(s)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

// If message confirmation is enabled then ensure that the confirmation message is
// received and dealt with
func confirmOne(confirms <-chan amqp.Confirmation) {
	logrus.Debug("Waiting for confirmation of publishing")
	if confirmed := <-confirms; confirmed.Ack {
		logrus.Debugf("Confirmed delivery with tag: %d", confirmed.DeliveryTag)
	} else {
		logrus.Debugf("Failed delivery with tag: %d", confirmed.DeliveryTag)
	}
}

// Take the log-driver options and put them into a data structure. Convert any values
// to the appropriate data type (eg string -> bool)
func parseURL(ctx logger.Context) (brokerArray []*amqpBroker) {
	amqpURLs := strings.Split(ctx.Config["amqp-url"], " ")
	for _, amqpURL := range amqpURLs {
		b, err := strconv.ParseBool(ctx.Config["amqp-confirm"])
		if err != nil {
			logrus.Errorf("Value %v is not valid: %v", b, err)
		} else {
			broker := &amqpBroker{
				BrokerURL:  amqpURL,
				CertPath:   ctx.Config["amqp-cert"],
				KeyPath:    ctx.Config["amqp-key"],
				Exchange:   ctx.Config["amqp-exchange"],
				Queue:      ctx.Config["amqp-queue"],
				RoutingKey: ctx.Config["amqp-routingkey"],
				Tag:        ctx.Config["amqp-tag"],
				Confirm:    b,
			}
			brokerArray = append(brokerArray, broker)
		}
	}
	return brokerArray
}

// Take the settings specified in the JSON configuration file and put them into a data
// structure. Convert any values to their required data types
func parseJSONFile(path string) (brokerArray []*amqpBroker) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		logrus.Errorf("Could not open AMQP settings file: %v", err)
		return nil
	}
	err = json.Unmarshal(file, &brokerArray)
	if err != nil {
		logrus.Errorf("Could not read JSON: %v", err)
		return nil
	}
	return brokerArray
}

// Cleanly close the connection with the broker.
func (s *amqpLogger) Close() error {
	logrus.Info("Closing connection")
	if s.connection != nil {
		if s.connection.c != nil {
			s.connection.c.Close()
		}
		if s.connection.conn != nil {
			s.connection.conn.Close()
		}
	}
	return nil
}

func (s *amqpLogger) Name() string {
	return name
}

// ValidateLogOpt checks for the amqp-specific log options
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "amqp-cert":
		case "amqp-key":
		case "amqp-url":
		case "amqp-exchange":
		case "amqp-queue":
		case "amqp-routingkey":
		case "amqp-tag":
		case "amqp-confirm":
		case "amqp-settings":
		default:
			return fmt.Errorf("unknown log opt '%s' for amqp log driver", key)
		}
	}
	return nil
}
