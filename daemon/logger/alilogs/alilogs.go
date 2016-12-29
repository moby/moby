// Package alilogs provides the logdriver for forwarding container logs to Ali Log Service

package alilogs

import (
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/galaxydi/go-loghub"
	"github.com/golang/protobuf/proto"
)

const (
	name        = "alilogs"
	endpointKey = "alilogs-endpoint"
	projectKey  = "alilogs-project"
	logstoreKey = "alilogs-logstore"

	topicEnvKey     = "topic"
	serviceEnvKey   = "serviceName"
	functionEnvKey  = "functionName"
	requestIDEnvKey = "requestID"

	accessKeyIDEnvKey     = "accessKeyID"
	accessKeySecretEnvKey = "accessKeySecret"
	sessionTokenEnvKey    = "sessionToken"

	batchPublishFrequency = 5 * time.Second

	//PutLogs接口每次可以写入的日志数据量上限为3MB或者4096条
	maximumBytesPerPut = 3145728
	maximumLogsPerPut  = 4096
)

type logStream struct {
	topic            string
	serviceName      string
	functionName     string
	requestID        string
	extraLogContents []*sls.LogContent
	client           AliLogAPI
	messages         chan *logger.Message
	lock             sync.RWMutex
	closed           bool
}

// init registers the alilogs driver
func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New creates an alilogs logger using the configuration passed in on the context
func New(ctx logger.Context) (logger.Logger, error) {
	endpoint := ctx.Config[endpointKey]
	projectName := ctx.Config[projectKey]
	logstoreName := ctx.Config[logstoreKey]
	extraContents := []*sls.LogContent{}
	accessKeyID := ""
	accessKeySecret := ""
	sessionToken := ""
	topicName := ""
	serviceName := ""
	functionName := ""
	requestID := ""

	extra := ctx.ExtraAttributes(nil)
	value, ok := extra[accessKeyIDEnvKey]
	if ok {
		accessKeyID = value
	} else {
		return nil, fmt.Errorf("must specify a value for env '%s'", accessKeyIDEnvKey)
	}
	value, ok = extra[accessKeySecretEnvKey]
	if ok {
		accessKeySecret = value
	} else {
		return nil, fmt.Errorf("must specify a value for env '%s'", accessKeySecretEnvKey)
	}

	if value, ok = extra[sessionTokenEnvKey]; ok {
		sessionToken = value
	}

	if value, ok = extra[topicEnvKey]; ok {
		topicName = value
	}

	// extra attributes in log record
	if value, ok = extra[serviceEnvKey]; ok {
		serviceName = value
		serviceNameContent := &sls.LogContent{
			Key:   proto.String(serviceEnvKey),
			Value: proto.String(serviceName),
		}
		extraContents = append(extraContents, serviceNameContent)
	}
	if value, ok = extra[functionEnvKey]; ok {
		functionName = value
		functionNameContent := &sls.LogContent{
			Key:   proto.String(functionEnvKey),
			Value: proto.String(functionName),
		}
		extraContents = append(extraContents, functionNameContent)
	}
	if value, ok = extra[requestIDEnvKey]; ok {
		requestID = value
		requestIDContent := &sls.LogContent{
			Key:   proto.String(requestIDEnvKey),
			Value: proto.String(requestID),
		}
		extraContents = append(extraContents, requestIDContent)
	}

	aliLogClient, err := NewAliLogClient(endpoint, projectName, logstoreName, accessKeyID, accessKeySecret, sessionToken)
	if err != nil {
		return nil, err
	}
	containerStream := &logStream{
		topic:            topicName,
		serviceName:      serviceName,
		functionName:     functionName,
		requestID:        requestID,
		extraLogContents: extraContents,
		client:           aliLogClient,
		messages:         make(chan *logger.Message, maximumLogsPerPut),
	}

	go containerStream.collectLogs()
	return containerStream, nil
}

// Name returns the name of ali logging driver
func (ls *logStream) Name() string {
	return name
}

// Log submits messages for logging by an instance of the alilogs logging driver
func (ls *logStream) Log(msg *logger.Message) error {
	ls.lock.RLock()
	defer ls.lock.RUnlock()
	if !ls.closed {
		// buffer up the data, making sure to copy the Line data
		ls.messages <- msg
	}
	return nil
}

// Close closes the instance of the alilogs logging driver
func (ls *logStream) Close() error {
	ls.lock.Lock()
	defer ls.lock.Unlock()
	if !ls.closed {
		close(ls.messages)
	}
	ls.closed = true
	return nil
}

// newTicker is used for time-based batching.  newTicker is a variable such
// that the implementation can be swapped out for unit tests.
var newTicker = func(freq time.Duration) *time.Ticker {
	return time.NewTicker(freq)
}

// PutLogs executes as a goroutine to perform put logs for
// submission to the logstore.  Batching is performed on time- and size-
// bases.  Time-based batching occurs at a 5 second interval (defined in the
// batchPublishFrequency const).  Size-based batching is performed on the
// maximum number of logs per batch (defined in maximumLogsPerPut) and
// the maximum number of total bytes in a batch (defined in
// maximumBytesPerPut).
func (ls *logStream) collectLogs() {
	aliLogClient := ls.client.(*AliLogClient)
	logGroup := sls.LogGroup{
		Topic: proto.String(ls.topic),
		Logs:  []*sls.Log{},
	}
	timer := newTicker(batchPublishFrequency)
	for {
		select {
		case <-timer.C:
			ls.publishLogs(&logGroup)
			logrus.WithFields(logrus.Fields{
				"endpoint":             aliLogClient.Endpoint,
				"project":              aliLogClient.ProjectName,
				"logstore":             aliLogClient.LogstoreName,
				"serviceName":          ls.serviceName,
				"functionName":         ls.functionName,
				"requestID":            ls.requestID,
				"published log number": len(logGroup.Logs),
				"published log size":   logGroup.Size(),
			}).Debug("publish log when timer timeout")
			logGroup.Reset()
			logGroup.Topic = proto.String(ls.topic)
		case msg, more := <-ls.messages:
			if !more {
				ls.publishLogs(&logGroup)
				logrus.WithFields(logrus.Fields{
					"endpoint":             aliLogClient.Endpoint,
					"project":              aliLogClient.ProjectName,
					"logstore":             aliLogClient.LogstoreName,
					"serviceName":          ls.serviceName,
					"functionName":         ls.functionName,
					"reuestID":             ls.requestID,
					"published log number": len(logGroup.Logs),
					"published log size":   logGroup.Size(),
				}).Debug("publish log when no more logs")
				return
			}
			unprocessedLine := msg.Line
			logMsg := &sls.LogContent{
				Key:   proto.String("message"),
				Value: proto.String(string(unprocessedLine)),
			}
			contents := ls.extraLogContents
			contents = append(contents, logMsg)
			logRecord := sls.Log{
				Time:     proto.Uint32(uint32(time.Now().Unix())),
				Contents: contents,
			}
			if len(unprocessedLine) > 0 {
				if (len(logGroup.Logs) >= maximumLogsPerPut) || (logGroup.Size()+logRecord.Size() > maximumBytesPerPut) {
					// Publish an existing batch if it's already over the maximum number of logs or if adding this
					// line would push it over the maximum number of total bytes.
					ls.publishLogs(&logGroup)
					logrus.WithFields(logrus.Fields{
						"endpoint":             aliLogClient.Endpoint,
						"project":              aliLogClient.ProjectName,
						"logstore":             aliLogClient.LogstoreName,
						"serviceName":          ls.serviceName,
						"functionName":         ls.functionName,
						"requestID":            ls.requestID,
						"published log number": len(logGroup.Logs),
						"published log size":   logGroup.Size(),
					}).Debug("publish logs when touch the limit")
					logGroup.Reset()
					logGroup.Topic = proto.String(ls.topic)
				}
				logGroup.Logs = append(logGroup.Logs, &logRecord)
			}
		}
	}
}

// publishLogs calls PutLogs for a given LogGroup
func (ls *logStream) publishLogs(lg *sls.LogGroup) {
	err := ls.client.PutLogs(lg)
	if err != nil {
		if serviceErr, ok := err.(sls.Error); ok {
			aliLogClient := ls.client.(*AliLogClient)
			logrus.WithFields(logrus.Fields{
				"errorCode":    serviceErr.Code,
				"errorMessage": serviceErr.Message,
				"endpoint":     aliLogClient.Endpoint,
				"project":      aliLogClient.ProjectName,
				"logstore":     aliLogClient.LogstoreName,
				"serviceName":  ls.serviceName,
				"functionName": ls.functionName,
				"requestId":    ls.requestID,
			}).Error("PutLogs occurs sls error")
		} else {
			logrus.Error(err)
		}
	}
}

// ValidateLogOpt looks for alilogs-specific log options
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "env":
		case endpointKey, projectKey, logstoreKey:
		default:
			return fmt.Errorf("unknown log opt '%s' for %s log driver", key, name)
		}
	}
	if cfg[endpointKey] == "" {
		return fmt.Errorf("must specify a value for log opt '%s'", endpointKey)
	}
	if cfg[projectKey] == "" {
		return fmt.Errorf("must specify a value for log opt '%s'", projectKey)
	}
	if cfg[logstoreKey] == "" {
		return fmt.Errorf("must specify a value for log opt '%s'", logstoreKey)
	}
	return nil
}
