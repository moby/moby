package omslogs

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/daemon/logger"
	client "github.com/docker/docker/daemon/logger/omslogs/omsclient"
	"github.com/sirupsen/logrus"
)

const (
	name = "omslogs"

	// Command line options
	optDomain      = "omslogs-domain"
	optWorkspaceID = "omslogs-workspaceID"
	optSharedKey   = "omslogs-sharedKey"
	optTimeout     = "omslogs-timeout"

	// Environment variables
	envDomain                = "OMS_LOGGING_DOMAIN"
	envTimeout               = "OMS_LOGGING_DRIVER_TIMEOUT"
	envPostMessagesFrequency = "OMS_LOGGING_DRIVER_POST_MESSAGES_FREQUENCY"
	envPostMessagesBatchSize = "OMS_LOGGING_DRIVER_POST_MESSAGES_BATCH_SIZE"
	envBufferSize            = "OMS_LOGGING_DRIVER_BUFFER_SIZE"
	envStreamChannelSize     = "OMS_LOGGING_DRIVER_CHANNEL_SIZE"

	// Default option values
	defaultTimeout               = time.Duration(5 * time.Second)
	defaultPostMessagesFrequency = time.Duration(5 * time.Second)
	defaultPostMessagesBatchSize = 100
	defaultBufferSize            = 10 * defaultPostMessagesBatchSize
	defaultStreamChannelSize     = 4 * defaultPostMessagesBatchSize

	// Errors
	errOptRequired = "must specify a value for log opt '%s'"
)

type omsLogger struct {
	// Initial container data
	containerID   string
	containerName string
	imageID       string
	imageName     string

	// Client options
	timeout               time.Duration
	postMessagesFrequency time.Duration
	postMessagesBatchSize int
	bufferSize            int
	streamChannelSize     int
	client                client.OmsLogClient

	// Synchronization
	stream    chan *omsMessage
	lock      sync.RWMutex
	closed    bool
	closedSig *sync.Cond
}

type omsMessage struct {
	ContainerID    string `json:"containerId"`
	ContainerName  string `json:"containerName"`
	ImageID        string `json:"imageId"`
	ImageName      string `json:"imageName"`
	TimeGenerated  string `json:"timeGenerated"`
	LogEntrySource string `json:"logEntrySource"`
	LogEntry       string `json:"logEntry"`
}

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}

	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// ValidateLogOpt looks for workspaceID and sharedKey.
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case optDomain:
		case optWorkspaceID:
		case optSharedKey:
		case optTimeout:
			if duration, err := getOptionDuration(cfg[key], defaultTimeout); err != nil {
				return err
			} else if duration.Nanoseconds() <= 0 {
				return fmt.Errorf("negative log opt '%s' value for %s log driver", key, name)
			}

			return nil
		default:
			return fmt.Errorf("unknown log opt '%s' for %s log driver", key, name)
		}
	}

	return nil
}

// New creates a new instance of the logging driver using configuration options passed in via the context.
func New(info logger.Info) (logger.Logger, error) {
	domain := info.Config[optDomain]
	workspaceID := info.Config[optWorkspaceID]
	sharedKey := info.Config[optSharedKey]

	if workspaceID == "" {
		return nil, fmt.Errorf(errOptRequired, workspaceID)
	}

	if sharedKey == "" {
		return nil, fmt.Errorf(errOptRequired, sharedKey)
	}

	l := &omsLogger{
		containerID:   info.ContainerID,
		containerName: info.ContainerName,
		imageID:       info.ContainerImageID,
		imageName:     info.ContainerImageName,
	}

	if domain == "" {
		domain = os.Getenv(envDomain)
	}

	l.timeout = defaultTimeout
	if timeoutStr, ok := info.Config[optTimeout]; ok {
		timeout, err := getOptionDuration(timeoutStr, defaultTimeout)
		if err != nil {
			logrus.Error(err)
		}

		l.timeout = timeout
	} else {
		l.timeout = getEnvOptionDuration(envTimeout, defaultTimeout)
	}

	l.postMessagesFrequency = getEnvOptionDuration(envPostMessagesFrequency, defaultPostMessagesFrequency)
	l.postMessagesBatchSize = getEnvOptionInt(envPostMessagesBatchSize, defaultPostMessagesBatchSize)
	l.bufferSize = getEnvOptionInt(envBufferSize, defaultBufferSize)
	l.streamChannelSize = getEnvOptionInt(envStreamChannelSize, defaultStreamChannelSize)

	l.client = client.NewOmsLogClient(domain, workspaceID, sharedKey, l.timeout)
	l.stream = make(chan *omsMessage, l.streamChannelSize)

	go l.work()
	return l, nil
}

// Log logs a message to the logging driver.
func (l *omsLogger) Log(message *logger.Message) error {
	msg := l.createMessage(message)

	logger.PutMessage(message)
	return l.queueMessage(msg)
}

// Name gets the name of the logging driver.
func (l *omsLogger) Name() string {
	return name
}

// Close closes the logging driver.
func (l *omsLogger) Close() error {
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.closedSig == nil {
		l.closedSig = sync.NewCond(&l.lock)

		close(l.stream)
		for !l.closed {
			l.closedSig.Wait()
		}
	}

	return nil
}

func (l *omsLogger) createMessage(m *logger.Message) *omsMessage {
	return &omsMessage{
		ContainerID:    l.containerID,
		ContainerName:  l.containerName,
		ImageID:        l.imageID,
		ImageName:      l.imageName,
		TimeGenerated:  m.Timestamp.Format(time.RFC3339),
		LogEntrySource: m.Source,
		LogEntry:       string(m.Line),
	}
}

func (l *omsLogger) processMessages(messages []*omsMessage, final bool) []*omsMessage {
	len := len(messages)
	for i := 0; i < len; i += l.postMessagesBatchSize {
		upper := i + l.postMessagesBatchSize
		if upper > len {
			upper = len
		}

		if err := l.sendMessages(messages[i:upper]); err != nil {
			logrus.Error(err)

			if len-1 >= l.bufferSize || final {
				// Send any remaining messages to daemon log.
				if final {
					upper = len
				}

				for j := 0; i < upper; j++ {
					if buffer, err := json.Marshal(messages[j]); err != nil {
						logrus.Error(err)
					} else {
						logrus.Errorf("Failed to send message: %s", string(buffer))
					}
				}

				return messages[upper:len]
			}

			// Return remaining buffer.
			return messages[i:len]
		}
	}

	// Return empty buffer when all messages sent.
	return messages[:0]
}

func (l *omsLogger) queueMessage(m *omsMessage) error {
	l.lock.RLock()
	defer l.lock.RUnlock()

	if l.closedSig != nil {
		return fmt.Errorf("%s driver is closed", name)
	}

	l.stream <- m
	return nil
}

func (l *omsLogger) sendMessages(messages []*omsMessage) error {
	// TODO: Send messages as a JSON array. Double check its supported.

	buffer, err := json.Marshal(messages)
	if err != nil {
		return err
	}

	// TODO: support setting mock client.OmsLogClient for testing.
	if err := l.client.PostData(&buffer, "ContainerLog"); err != nil {
		return err
	}

	return nil
}

func (l *omsLogger) setClient(c client.OmsLogClient) {
	l.client = c
}

func (l *omsLogger) work() {
	timer := time.NewTicker(l.postMessagesFrequency)
	var messages []*omsMessage

	for {
		select {
		case message, open := <-l.stream:
			if !open {
				l.processMessages(messages, true)

				l.lock.Lock()
				defer l.lock.Unlock()

				l.closed = true
				l.closedSig.Signal()

				return
			}

			messages = append(messages, message)
			if len(messages)%l.postMessagesBatchSize == 0 {
				messages = l.processMessages(messages, false)
			}

		case <-timer.C:
			messages = l.processMessages(messages, false)
		}
	}
}

func getOptionDuration(valueStr string, defaultValue time.Duration) (time.Duration, error) {
	if valueStr == "" {
		return defaultValue, nil
	}

	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue, fmt.Errorf("invalid duration: %s. Using default value: %v", valueStr, defaultValue)
	}

	return value, nil
}

func getEnvOptionDuration(envName string, defaultValue time.Duration) time.Duration {
	value, err := getOptionDuration(os.Getenv(envName), defaultValue)
	if err != nil {
		logrus.Errorf("Failed to parse environment variable '%s': %s", envName, err.Error())
	}

	return value
}

func getOptionInt(valueStr string, defaultValue int) (int, error) {
	if valueStr == "" {
		return defaultValue, nil
	}

	value, err := strconv.ParseInt(valueStr, 10, 32)
	if err != nil {
		return defaultValue, fmt.Errorf("invalid integer: %s. Using default value: %v", valueStr, defaultValue)
	}

	if value < 0 {
		return defaultValue, fmt.Errorf("negative integer not supported: %s. Using default value %v", valueStr, defaultValue)
	}

	if value > math.MaxInt32 {
		return defaultValue, fmt.Errorf("integer overflow: %s. Using default value %v", valueStr, defaultValue)
	}

	return int(value), nil
}

func getEnvOptionInt(envName string, defaultValue int) int {
	value, err := getOptionInt(os.Getenv(envName), defaultValue)
	if err != nil {
		logrus.Errorf("Failed to parse environment variable '%s': %s", envName, err.Error())
	}

	return value
}
