// Package awslogs provides the logdriver for forwarding container logs to Amazon CloudWatch Logs
package awslogs

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/docker/docker/dockerversion"
)

const (
	name                  = "awslogs"
	regionKey             = "awslogs-region"
	regionEnvKey          = "AWS_REGION"
	logGroupKey           = "awslogs-group"
	logStreamKey          = "awslogs-stream"
	tagKey                = "tag"
	batchPublishFrequency = 5 * time.Second

	// See: http://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
	perEventBytes          = 26
	maximumBytesPerPut     = 1048576
	maximumLogEventsPerPut = 10000

	// See: http://docs.aws.amazon.com/AmazonCloudWatch/latest/DeveloperGuide/cloudwatch_limits.html
	maximumBytesPerEvent = 262144 - perEventBytes

	resourceAlreadyExistsCode = "ResourceAlreadyExistsException"
	dataAlreadyAcceptedCode   = "DataAlreadyAcceptedException"
	invalidSequenceTokenCode  = "InvalidSequenceTokenException"

	userAgentHeader = "User-Agent"
)

type logStream struct {
	logStreamName string
	logGroupName  string
	client        api
	messages      chan *logger.Message
	lock          sync.RWMutex
	closed        bool
	sequenceToken *string
}

type api interface {
	CreateLogStream(*cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error)
	PutLogEvents(*cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error)
}

type regionFinder interface {
	Region() (string, error)
}

type wrappedEvent struct {
	inputLogEvent *cloudwatchlogs.InputLogEvent
	insertOrder   int
}
type byTimestamp []wrappedEvent

// init registers the awslogs driver
func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New creates an awslogs logger using the configuration passed in on the
// context.  Supported context configuration variables are awslogs-region,
// awslogs-group, and awslogs-stream.  When available, configuration is
// also taken from environment variables AWS_REGION, AWS_ACCESS_KEY_ID,
// AWS_SECRET_ACCESS_KEY, the shared credentials file (~/.aws/credentials), and
// the EC2 Instance Metadata Service.
func New(ctx logger.Context) (logger.Logger, error) {
	logGroupName := ctx.Config[logGroupKey]
	logStreamName, err := loggerutils.ParseLogTag(ctx, "{{.FullID}}")
	if err != nil {
		return nil, err
	}

	if ctx.Config[logStreamKey] != "" {
		logStreamName = ctx.Config[logStreamKey]
	}
	client, err := newAWSLogsClient(ctx)
	if err != nil {
		return nil, err
	}
	containerStream := &logStream{
		logStreamName: logStreamName,
		logGroupName:  logGroupName,
		client:        client,
		messages:      make(chan *logger.Message, 4096),
	}
	err = containerStream.create()
	if err != nil {
		return nil, err
	}
	go containerStream.collectBatch()

	return containerStream, nil
}

// newRegionFinder is a variable such that the implementation
// can be swapped out for unit tests.
var newRegionFinder = func() regionFinder {
	return ec2metadata.New(session.New())
}

// newAWSLogsClient creates the service client for Amazon CloudWatch Logs.
// Customizations to the default client from the SDK include a Docker-specific
// User-Agent string and automatic region detection using the EC2 Instance
// Metadata Service when region is otherwise unspecified.
func newAWSLogsClient(ctx logger.Context) (api, error) {
	var region *string
	if os.Getenv(regionEnvKey) != "" {
		region = aws.String(os.Getenv(regionEnvKey))
	}
	if ctx.Config[regionKey] != "" {
		region = aws.String(ctx.Config[regionKey])
	}
	if region == nil || *region == "" {
		logrus.Info("Trying to get region from EC2 Metadata")
		ec2MetadataClient := newRegionFinder()
		r, err := ec2MetadataClient.Region()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("Could not get region from EC2 metadata, environment, or log option")
			return nil, errors.New("Cannot determine region for awslogs driver")
		}
		region = &r
	}
	logrus.WithFields(logrus.Fields{
		"region": *region,
	}).Debug("Created awslogs client")

	client := cloudwatchlogs.New(session.New(), aws.NewConfig().WithRegion(*region))

	client.Handlers.Build.PushBackNamed(request.NamedHandler{
		Name: "DockerUserAgentHandler",
		Fn: func(r *request.Request) {
			currentAgent := r.HTTPRequest.Header.Get(userAgentHeader)
			r.HTTPRequest.Header.Set(userAgentHeader,
				fmt.Sprintf("Docker %s (%s) %s",
					dockerversion.Version, runtime.GOOS, currentAgent))
		},
	})
	return client, nil
}

// Name returns the name of the awslogs logging driver
func (l *logStream) Name() string {
	return name
}

// Log submits messages for logging by an instance of the awslogs logging driver
func (l *logStream) Log(msg *logger.Message) error {
	l.lock.RLock()
	defer l.lock.RUnlock()
	if !l.closed {
		// buffer up the data, making sure to copy the Line data
		l.messages <- logger.CopyMessage(msg)
	}
	return nil
}

// Close closes the instance of the awslogs logging driver
func (l *logStream) Close() error {
	l.lock.Lock()
	defer l.lock.Unlock()
	if !l.closed {
		close(l.messages)
	}
	l.closed = true
	return nil
}

// create creates a log stream for the instance of the awslogs logging driver
func (l *logStream) create() error {
	input := &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(l.logGroupName),
		LogStreamName: aws.String(l.logStreamName),
	}

	_, err := l.client.CreateLogStream(input)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			fields := logrus.Fields{
				"errorCode":     awsErr.Code(),
				"message":       awsErr.Message(),
				"origError":     awsErr.OrigErr(),
				"logGroupName":  l.logGroupName,
				"logStreamName": l.logStreamName,
			}
			if awsErr.Code() == resourceAlreadyExistsCode {
				// Allow creation to succeed
				logrus.WithFields(fields).Info("Log stream already exists")
				return nil
			}
			logrus.WithFields(fields).Error("Failed to create log stream")
		}
	}
	return err
}

// newTicker is used for time-based batching.  newTicker is a variable such
// that the implementation can be swapped out for unit tests.
var newTicker = func(freq time.Duration) *time.Ticker {
	return time.NewTicker(freq)
}

// collectBatch executes as a goroutine to perform batching of log events for
// submission to the log stream.  Batching is performed on time- and size-
// bases.  Time-based batching occurs at a 5 second interval (defined in the
// batchPublishFrequency const).  Size-based batching is performed on the
// maximum number of events per batch (defined in maximumLogEventsPerPut) and
// the maximum number of total bytes in a batch (defined in
// maximumBytesPerPut).  Log messages are split by the maximum bytes per event
// (defined in maximumBytesPerEvent).  There is a fixed per-event byte overhead
// (defined in perEventBytes) which is accounted for in split- and batch-
// calculations.
func (l *logStream) collectBatch() {
	timer := newTicker(batchPublishFrequency)
	var events []wrappedEvent
	bytes := 0
	for {
		select {
		case <-timer.C:
			l.publishBatch(events)
			events = events[:0]
			bytes = 0
		case msg, more := <-l.messages:
			if !more {
				l.publishBatch(events)
				return
			}
			unprocessedLine := msg.Line
			for len(unprocessedLine) > 0 {
				// Split line length so it does not exceed the maximum
				lineBytes := len(unprocessedLine)
				if lineBytes > maximumBytesPerEvent {
					lineBytes = maximumBytesPerEvent
				}
				line := unprocessedLine[:lineBytes]
				unprocessedLine = unprocessedLine[lineBytes:]
				if (len(events) >= maximumLogEventsPerPut) || (bytes+lineBytes+perEventBytes > maximumBytesPerPut) {
					// Publish an existing batch if it's already over the maximum number of events or if adding this
					// event would push it over the maximum number of total bytes.
					l.publishBatch(events)
					events = events[:0]
					bytes = 0
				}
				events = append(events, wrappedEvent{
					inputLogEvent: &cloudwatchlogs.InputLogEvent{
						Message:   aws.String(string(line)),
						Timestamp: aws.Int64(msg.Timestamp.UnixNano() / int64(time.Millisecond)),
					},
					insertOrder: len(events),
				})
				bytes += (lineBytes + perEventBytes)
			}
		}
	}
}

// publishBatch calls PutLogEvents for a given set of InputLogEvents,
// accounting for sequencing requirements (each request must reference the
// sequence token returned by the previous request).
func (l *logStream) publishBatch(events []wrappedEvent) {
	if len(events) == 0 {
		return
	}

	// events in a batch must be sorted by timestamp
	// see http://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
	sort.Sort(byTimestamp(events))
	cwEvents := unwrapEvents(events)

	nextSequenceToken, err := l.putLogEvents(cwEvents, l.sequenceToken)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == dataAlreadyAcceptedCode {
				// already submitted, just grab the correct sequence token
				parts := strings.Split(awsErr.Message(), " ")
				nextSequenceToken = &parts[len(parts)-1]
				logrus.WithFields(logrus.Fields{
					"errorCode":     awsErr.Code(),
					"message":       awsErr.Message(),
					"logGroupName":  l.logGroupName,
					"logStreamName": l.logStreamName,
				}).Info("Data already accepted, ignoring error")
				err = nil
			} else if awsErr.Code() == invalidSequenceTokenCode {
				// sequence code is bad, grab the correct one and retry
				parts := strings.Split(awsErr.Message(), " ")
				token := parts[len(parts)-1]
				nextSequenceToken, err = l.putLogEvents(cwEvents, &token)
			}
		}
	}
	if err != nil {
		logrus.Error(err)
	} else {
		l.sequenceToken = nextSequenceToken
	}
}

// putLogEvents wraps the PutLogEvents API
func (l *logStream) putLogEvents(events []*cloudwatchlogs.InputLogEvent, sequenceToken *string) (*string, error) {
	input := &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     events,
		SequenceToken: sequenceToken,
		LogGroupName:  aws.String(l.logGroupName),
		LogStreamName: aws.String(l.logStreamName),
	}
	resp, err := l.client.PutLogEvents(input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			logrus.WithFields(logrus.Fields{
				"errorCode":     awsErr.Code(),
				"message":       awsErr.Message(),
				"origError":     awsErr.OrigErr(),
				"logGroupName":  l.logGroupName,
				"logStreamName": l.logStreamName,
			}).Error("Failed to put log events")
		}
		return nil, err
	}
	return resp.NextSequenceToken, nil
}

// ValidateLogOpt looks for awslogs-specific log options awslogs-region,
// awslogs-group, and awslogs-stream
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case logGroupKey:
		case logStreamKey:
		case regionKey:
		case tagKey:
		default:
			return fmt.Errorf("unknown log opt '%s' for %s log driver", key, name)
		}
	}
	if cfg[logGroupKey] == "" {
		return fmt.Errorf("must specify a value for log opt '%s'", logGroupKey)
	}
	return nil
}

// Len returns the length of a byTimestamp slice.  Len is required by the
// sort.Interface interface.
func (slice byTimestamp) Len() int {
	return len(slice)
}

// Less compares two values in a byTimestamp slice by Timestamp.  Less is
// required by the sort.Interface interface.
func (slice byTimestamp) Less(i, j int) bool {
	iTimestamp, jTimestamp := int64(0), int64(0)
	if slice != nil && slice[i].inputLogEvent.Timestamp != nil {
		iTimestamp = *slice[i].inputLogEvent.Timestamp
	}
	if slice != nil && slice[j].inputLogEvent.Timestamp != nil {
		jTimestamp = *slice[j].inputLogEvent.Timestamp
	}
	if iTimestamp == jTimestamp {
		return slice[i].insertOrder < slice[j].insertOrder
	}
	return iTimestamp < jTimestamp
}

// Swap swaps two values in a byTimestamp slice with each other.  Swap is
// required by the sort.Interface interface.
func (slice byTimestamp) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func unwrapEvents(events []wrappedEvent) []*cloudwatchlogs.InputLogEvent {
	cwEvents := []*cloudwatchlogs.InputLogEvent{}
	for _, input := range events {
		cwEvents = append(cwEvents, input.inputLogEvent)
	}
	return cwEvents
}
