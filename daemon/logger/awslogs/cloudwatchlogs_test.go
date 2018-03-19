package awslogs // import "github.com/docker/docker/daemon/logger/awslogs"

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/docker/docker/dockerversion"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

const (
	groupName         = "groupName"
	streamName        = "streamName"
	sequenceToken     = "sequenceToken"
	nextSequenceToken = "nextSequenceToken"
	logline           = "this is a log line\r"
	multilineLogline  = "2017-01-01 01:01:44 This is a multiline log entry\r"
)

// Generates i multi-line events each with j lines
func (l *logStream) logGenerator(lineCount int, multilineCount int) {
	for i := 0; i < multilineCount; i++ {
		l.Log(&logger.Message{
			Line:      []byte(multilineLogline),
			Timestamp: time.Time{},
		})
		for j := 0; j < lineCount; j++ {
			l.Log(&logger.Message{
				Line:      []byte(logline),
				Timestamp: time.Time{},
			})
		}
	}
}

func testEventBatch(events []wrappedEvent) *eventBatch {
	batch := newEventBatch()
	for _, event := range events {
		eventlen := len([]byte(*event.inputLogEvent.Message))
		batch.add(event, eventlen)
	}
	return batch
}

func TestNewAWSLogsClientUserAgentHandler(t *testing.T) {
	info := logger.Info{
		Config: map[string]string{
			regionKey: "us-east-1",
		},
	}

	client, err := newAWSLogsClient(info)
	if err != nil {
		t.Fatal(err)
	}
	realClient, ok := client.(*cloudwatchlogs.CloudWatchLogs)
	if !ok {
		t.Fatal("Could not cast client to cloudwatchlogs.CloudWatchLogs")
	}
	buildHandlerList := realClient.Handlers.Build
	request := &request.Request{
		HTTPRequest: &http.Request{
			Header: http.Header{},
		},
	}
	buildHandlerList.Run(request)
	expectedUserAgentString := fmt.Sprintf("Docker %s (%s) %s/%s (%s; %s; %s)",
		dockerversion.Version, runtime.GOOS, aws.SDKName, aws.SDKVersion, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	userAgent := request.HTTPRequest.Header.Get("User-Agent")
	if userAgent != expectedUserAgentString {
		t.Errorf("Wrong User-Agent string, expected \"%s\" but was \"%s\"",
			expectedUserAgentString, userAgent)
	}
}

func TestNewAWSLogsClientRegionDetect(t *testing.T) {
	info := logger.Info{
		Config: map[string]string{},
	}

	mockMetadata := newMockMetadataClient()
	newRegionFinder = func() regionFinder {
		return mockMetadata
	}
	mockMetadata.regionResult <- &regionResult{
		successResult: "us-east-1",
	}

	_, err := newAWSLogsClient(info)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateSuccess(t *testing.T) {
	mockClient := newMockClient()
	stream := &logStream{
		client:        mockClient,
		logGroupName:  groupName,
		logStreamName: streamName,
	}
	mockClient.createLogStreamResult <- &createLogStreamResult{}

	err := stream.create()

	if err != nil {
		t.Errorf("Received unexpected err: %v\n", err)
	}
	argument := <-mockClient.createLogStreamArgument
	if argument.LogGroupName == nil {
		t.Fatal("Expected non-nil LogGroupName")
	}
	if *argument.LogGroupName != groupName {
		t.Errorf("Expected LogGroupName to be %s", groupName)
	}
	if argument.LogStreamName == nil {
		t.Fatal("Expected non-nil LogStreamName")
	}
	if *argument.LogStreamName != streamName {
		t.Errorf("Expected LogStreamName to be %s", streamName)
	}
}

func TestCreateLogGroupSuccess(t *testing.T) {
	mockClient := newMockClient()
	stream := &logStream{
		client:         mockClient,
		logGroupName:   groupName,
		logStreamName:  streamName,
		logCreateGroup: true,
	}
	mockClient.createLogGroupResult <- &createLogGroupResult{}
	mockClient.createLogStreamResult <- &createLogStreamResult{}

	err := stream.create()

	if err != nil {
		t.Errorf("Received unexpected err: %v\n", err)
	}
	argument := <-mockClient.createLogStreamArgument
	if argument.LogGroupName == nil {
		t.Fatal("Expected non-nil LogGroupName")
	}
	if *argument.LogGroupName != groupName {
		t.Errorf("Expected LogGroupName to be %s", groupName)
	}
	if argument.LogStreamName == nil {
		t.Fatal("Expected non-nil LogStreamName")
	}
	if *argument.LogStreamName != streamName {
		t.Errorf("Expected LogStreamName to be %s", streamName)
	}
}

func TestCreateError(t *testing.T) {
	mockClient := newMockClient()
	stream := &logStream{
		client: mockClient,
	}
	mockClient.createLogStreamResult <- &createLogStreamResult{
		errorResult: errors.New("Error"),
	}

	err := stream.create()

	if err == nil {
		t.Fatal("Expected non-nil err")
	}
}

func TestCreateAlreadyExists(t *testing.T) {
	mockClient := newMockClient()
	stream := &logStream{
		client: mockClient,
	}
	mockClient.createLogStreamResult <- &createLogStreamResult{
		errorResult: awserr.New(resourceAlreadyExistsCode, "", nil),
	}

	err := stream.create()

	if err != nil {
		t.Fatal("Expected nil err")
	}
}

func TestPublishBatchSuccess(t *testing.T) {
	mockClient := newMockClient()
	stream := &logStream{
		client:        mockClient,
		logGroupName:  groupName,
		logStreamName: streamName,
		sequenceToken: aws.String(sequenceToken),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		successResult: &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: aws.String(nextSequenceToken),
		},
	}
	events := []wrappedEvent{
		{
			inputLogEvent: &cloudwatchlogs.InputLogEvent{
				Message: aws.String(logline),
			},
		},
	}

	stream.publishBatch(testEventBatch(events))
	if stream.sequenceToken == nil {
		t.Fatal("Expected non-nil sequenceToken")
	}
	if *stream.sequenceToken != nextSequenceToken {
		t.Errorf("Expected sequenceToken to be %s, but was %s", nextSequenceToken, *stream.sequenceToken)
	}
	argument := <-mockClient.putLogEventsArgument
	if argument == nil {
		t.Fatal("Expected non-nil PutLogEventsInput")
	}
	if argument.SequenceToken == nil {
		t.Fatal("Expected non-nil PutLogEventsInput.SequenceToken")
	}
	if *argument.SequenceToken != sequenceToken {
		t.Errorf("Expected PutLogEventsInput.SequenceToken to be %s, but was %s", sequenceToken, *argument.SequenceToken)
	}
	if len(argument.LogEvents) != 1 {
		t.Errorf("Expected LogEvents to contain 1 element, but contains %d", len(argument.LogEvents))
	}
	if argument.LogEvents[0] != events[0].inputLogEvent {
		t.Error("Expected event to equal input")
	}
}

func TestPublishBatchError(t *testing.T) {
	mockClient := newMockClient()
	stream := &logStream{
		client:        mockClient,
		logGroupName:  groupName,
		logStreamName: streamName,
		sequenceToken: aws.String(sequenceToken),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		errorResult: errors.New("Error"),
	}

	events := []wrappedEvent{
		{
			inputLogEvent: &cloudwatchlogs.InputLogEvent{
				Message: aws.String(logline),
			},
		},
	}

	stream.publishBatch(testEventBatch(events))
	if stream.sequenceToken == nil {
		t.Fatal("Expected non-nil sequenceToken")
	}
	if *stream.sequenceToken != sequenceToken {
		t.Errorf("Expected sequenceToken to be %s, but was %s", sequenceToken, *stream.sequenceToken)
	}
}

func TestPublishBatchInvalidSeqSuccess(t *testing.T) {
	mockClient := newMockClientBuffered(2)
	stream := &logStream{
		client:        mockClient,
		logGroupName:  groupName,
		logStreamName: streamName,
		sequenceToken: aws.String(sequenceToken),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		errorResult: awserr.New(invalidSequenceTokenCode, "use token token", nil),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		successResult: &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: aws.String(nextSequenceToken),
		},
	}

	events := []wrappedEvent{
		{
			inputLogEvent: &cloudwatchlogs.InputLogEvent{
				Message: aws.String(logline),
			},
		},
	}

	stream.publishBatch(testEventBatch(events))
	if stream.sequenceToken == nil {
		t.Fatal("Expected non-nil sequenceToken")
	}
	if *stream.sequenceToken != nextSequenceToken {
		t.Errorf("Expected sequenceToken to be %s, but was %s", nextSequenceToken, *stream.sequenceToken)
	}

	argument := <-mockClient.putLogEventsArgument
	if argument == nil {
		t.Fatal("Expected non-nil PutLogEventsInput")
	}
	if argument.SequenceToken == nil {
		t.Fatal("Expected non-nil PutLogEventsInput.SequenceToken")
	}
	if *argument.SequenceToken != sequenceToken {
		t.Errorf("Expected PutLogEventsInput.SequenceToken to be %s, but was %s", sequenceToken, *argument.SequenceToken)
	}
	if len(argument.LogEvents) != 1 {
		t.Errorf("Expected LogEvents to contain 1 element, but contains %d", len(argument.LogEvents))
	}
	if argument.LogEvents[0] != events[0].inputLogEvent {
		t.Error("Expected event to equal input")
	}

	argument = <-mockClient.putLogEventsArgument
	if argument == nil {
		t.Fatal("Expected non-nil PutLogEventsInput")
	}
	if argument.SequenceToken == nil {
		t.Fatal("Expected non-nil PutLogEventsInput.SequenceToken")
	}
	if *argument.SequenceToken != "token" {
		t.Errorf("Expected PutLogEventsInput.SequenceToken to be %s, but was %s", "token", *argument.SequenceToken)
	}
	if len(argument.LogEvents) != 1 {
		t.Errorf("Expected LogEvents to contain 1 element, but contains %d", len(argument.LogEvents))
	}
	if argument.LogEvents[0] != events[0].inputLogEvent {
		t.Error("Expected event to equal input")
	}
}

func TestPublishBatchAlreadyAccepted(t *testing.T) {
	mockClient := newMockClient()
	stream := &logStream{
		client:        mockClient,
		logGroupName:  groupName,
		logStreamName: streamName,
		sequenceToken: aws.String(sequenceToken),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		errorResult: awserr.New(dataAlreadyAcceptedCode, "use token token", nil),
	}

	events := []wrappedEvent{
		{
			inputLogEvent: &cloudwatchlogs.InputLogEvent{
				Message: aws.String(logline),
			},
		},
	}

	stream.publishBatch(testEventBatch(events))
	if stream.sequenceToken == nil {
		t.Fatal("Expected non-nil sequenceToken")
	}
	if *stream.sequenceToken != "token" {
		t.Errorf("Expected sequenceToken to be %s, but was %s", "token", *stream.sequenceToken)
	}

	argument := <-mockClient.putLogEventsArgument
	if argument == nil {
		t.Fatal("Expected non-nil PutLogEventsInput")
	}
	if argument.SequenceToken == nil {
		t.Fatal("Expected non-nil PutLogEventsInput.SequenceToken")
	}
	if *argument.SequenceToken != sequenceToken {
		t.Errorf("Expected PutLogEventsInput.SequenceToken to be %s, but was %s", sequenceToken, *argument.SequenceToken)
	}
	if len(argument.LogEvents) != 1 {
		t.Errorf("Expected LogEvents to contain 1 element, but contains %d", len(argument.LogEvents))
	}
	if argument.LogEvents[0] != events[0].inputLogEvent {
		t.Error("Expected event to equal input")
	}
}

func TestCollectBatchSimple(t *testing.T) {
	mockClient := newMockClient()
	stream := &logStream{
		client:        mockClient,
		logGroupName:  groupName,
		logStreamName: streamName,
		sequenceToken: aws.String(sequenceToken),
		messages:      make(chan *logger.Message),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		successResult: &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: aws.String(nextSequenceToken),
		},
	}
	ticks := make(chan time.Time)
	newTicker = func(_ time.Duration) *time.Ticker {
		return &time.Ticker{
			C: ticks,
		}
	}

	go stream.collectBatch()

	stream.Log(&logger.Message{
		Line:      []byte(logline),
		Timestamp: time.Time{},
	})

	ticks <- time.Time{}
	stream.Close()

	argument := <-mockClient.putLogEventsArgument
	if argument == nil {
		t.Fatal("Expected non-nil PutLogEventsInput")
	}
	if len(argument.LogEvents) != 1 {
		t.Errorf("Expected LogEvents to contain 1 element, but contains %d", len(argument.LogEvents))
	}
	if *argument.LogEvents[0].Message != logline {
		t.Errorf("Expected message to be %s but was %s", logline, *argument.LogEvents[0].Message)
	}
}

func TestCollectBatchTicker(t *testing.T) {
	mockClient := newMockClient()
	stream := &logStream{
		client:        mockClient,
		logGroupName:  groupName,
		logStreamName: streamName,
		sequenceToken: aws.String(sequenceToken),
		messages:      make(chan *logger.Message),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		successResult: &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: aws.String(nextSequenceToken),
		},
	}
	ticks := make(chan time.Time)
	newTicker = func(_ time.Duration) *time.Ticker {
		return &time.Ticker{
			C: ticks,
		}
	}

	go stream.collectBatch()

	stream.Log(&logger.Message{
		Line:      []byte(logline + " 1"),
		Timestamp: time.Time{},
	})
	stream.Log(&logger.Message{
		Line:      []byte(logline + " 2"),
		Timestamp: time.Time{},
	})

	ticks <- time.Time{}

	// Verify first batch
	argument := <-mockClient.putLogEventsArgument
	if argument == nil {
		t.Fatal("Expected non-nil PutLogEventsInput")
	}
	if len(argument.LogEvents) != 2 {
		t.Errorf("Expected LogEvents to contain 2 elements, but contains %d", len(argument.LogEvents))
	}
	if *argument.LogEvents[0].Message != logline+" 1" {
		t.Errorf("Expected message to be %s but was %s", logline+" 1", *argument.LogEvents[0].Message)
	}
	if *argument.LogEvents[1].Message != logline+" 2" {
		t.Errorf("Expected message to be %s but was %s", logline+" 2", *argument.LogEvents[0].Message)
	}

	stream.Log(&logger.Message{
		Line:      []byte(logline + " 3"),
		Timestamp: time.Time{},
	})

	ticks <- time.Time{}
	argument = <-mockClient.putLogEventsArgument
	if argument == nil {
		t.Fatal("Expected non-nil PutLogEventsInput")
	}
	if len(argument.LogEvents) != 1 {
		t.Errorf("Expected LogEvents to contain 1 elements, but contains %d", len(argument.LogEvents))
	}
	if *argument.LogEvents[0].Message != logline+" 3" {
		t.Errorf("Expected message to be %s but was %s", logline+" 3", *argument.LogEvents[0].Message)
	}

	stream.Close()

}

func TestCollectBatchMultilinePattern(t *testing.T) {
	mockClient := newMockClient()
	multilinePattern := regexp.MustCompile("xxxx")
	stream := &logStream{
		client:           mockClient,
		logGroupName:     groupName,
		logStreamName:    streamName,
		multilinePattern: multilinePattern,
		sequenceToken:    aws.String(sequenceToken),
		messages:         make(chan *logger.Message),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		successResult: &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: aws.String(nextSequenceToken),
		},
	}
	ticks := make(chan time.Time)
	newTicker = func(_ time.Duration) *time.Ticker {
		return &time.Ticker{
			C: ticks,
		}
	}

	go stream.collectBatch()

	stream.Log(&logger.Message{
		Line:      []byte(logline),
		Timestamp: time.Now(),
	})
	stream.Log(&logger.Message{
		Line:      []byte(logline),
		Timestamp: time.Now(),
	})
	stream.Log(&logger.Message{
		Line:      []byte("xxxx " + logline),
		Timestamp: time.Now(),
	})

	ticks <- time.Now()

	// Verify single multiline event
	argument := <-mockClient.putLogEventsArgument
	assert.Check(t, argument != nil, "Expected non-nil PutLogEventsInput")
	assert.Check(t, is.Equal(1, len(argument.LogEvents)), "Expected single multiline event")
	assert.Check(t, is.Equal(logline+"\n"+logline+"\n", *argument.LogEvents[0].Message), "Received incorrect multiline message")

	stream.Close()

	// Verify single event
	argument = <-mockClient.putLogEventsArgument
	assert.Check(t, argument != nil, "Expected non-nil PutLogEventsInput")
	assert.Check(t, is.Equal(1, len(argument.LogEvents)), "Expected single multiline event")
	assert.Check(t, is.Equal("xxxx "+logline+"\n", *argument.LogEvents[0].Message), "Received incorrect multiline message")
}

func BenchmarkCollectBatch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mockClient := newMockClient()
		stream := &logStream{
			client:        mockClient,
			logGroupName:  groupName,
			logStreamName: streamName,
			sequenceToken: aws.String(sequenceToken),
			messages:      make(chan *logger.Message),
		}
		mockClient.putLogEventsResult <- &putLogEventsResult{
			successResult: &cloudwatchlogs.PutLogEventsOutput{
				NextSequenceToken: aws.String(nextSequenceToken),
			},
		}
		ticks := make(chan time.Time)
		newTicker = func(_ time.Duration) *time.Ticker {
			return &time.Ticker{
				C: ticks,
			}
		}

		go stream.collectBatch()
		stream.logGenerator(10, 100)
		ticks <- time.Time{}
		stream.Close()
	}
}

func BenchmarkCollectBatchMultilinePattern(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mockClient := newMockClient()
		multilinePattern := regexp.MustCompile(`\d{4}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[1,2][0-9]|3[0,1]) (?:[0,1][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]`)
		stream := &logStream{
			client:           mockClient,
			logGroupName:     groupName,
			logStreamName:    streamName,
			multilinePattern: multilinePattern,
			sequenceToken:    aws.String(sequenceToken),
			messages:         make(chan *logger.Message),
		}
		mockClient.putLogEventsResult <- &putLogEventsResult{
			successResult: &cloudwatchlogs.PutLogEventsOutput{
				NextSequenceToken: aws.String(nextSequenceToken),
			},
		}
		ticks := make(chan time.Time)
		newTicker = func(_ time.Duration) *time.Ticker {
			return &time.Ticker{
				C: ticks,
			}
		}
		go stream.collectBatch()
		stream.logGenerator(10, 100)
		ticks <- time.Time{}
		stream.Close()
	}
}

func TestCollectBatchMultilinePatternMaxEventAge(t *testing.T) {
	mockClient := newMockClient()
	multilinePattern := regexp.MustCompile("xxxx")
	stream := &logStream{
		client:           mockClient,
		logGroupName:     groupName,
		logStreamName:    streamName,
		multilinePattern: multilinePattern,
		sequenceToken:    aws.String(sequenceToken),
		messages:         make(chan *logger.Message),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		successResult: &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: aws.String(nextSequenceToken),
		},
	}
	ticks := make(chan time.Time)
	newTicker = func(_ time.Duration) *time.Ticker {
		return &time.Ticker{
			C: ticks,
		}
	}

	go stream.collectBatch()

	stream.Log(&logger.Message{
		Line:      []byte(logline),
		Timestamp: time.Now(),
	})

	// Log an event 1 second later
	stream.Log(&logger.Message{
		Line:      []byte(logline),
		Timestamp: time.Now().Add(time.Second),
	})

	// Fire ticker batchPublishFrequency seconds later
	ticks <- time.Now().Add(batchPublishFrequency + time.Second)

	// Verify single multiline event is flushed after maximum event buffer age (batchPublishFrequency)
	argument := <-mockClient.putLogEventsArgument
	assert.Check(t, argument != nil, "Expected non-nil PutLogEventsInput")
	assert.Check(t, is.Equal(1, len(argument.LogEvents)), "Expected single multiline event")
	assert.Check(t, is.Equal(logline+"\n"+logline+"\n", *argument.LogEvents[0].Message), "Received incorrect multiline message")

	// Log an event 1 second later
	stream.Log(&logger.Message{
		Line:      []byte(logline),
		Timestamp: time.Now().Add(time.Second),
	})

	// Fire ticker another batchPublishFrequency seconds later
	ticks <- time.Now().Add(2*batchPublishFrequency + time.Second)

	// Verify the event buffer is truly flushed - we should only receive a single event
	argument = <-mockClient.putLogEventsArgument
	assert.Check(t, argument != nil, "Expected non-nil PutLogEventsInput")
	assert.Check(t, is.Equal(1, len(argument.LogEvents)), "Expected single multiline event")
	assert.Check(t, is.Equal(logline+"\n", *argument.LogEvents[0].Message), "Received incorrect multiline message")
	stream.Close()
}

func TestCollectBatchMultilinePatternNegativeEventAge(t *testing.T) {
	mockClient := newMockClient()
	multilinePattern := regexp.MustCompile("xxxx")
	stream := &logStream{
		client:           mockClient,
		logGroupName:     groupName,
		logStreamName:    streamName,
		multilinePattern: multilinePattern,
		sequenceToken:    aws.String(sequenceToken),
		messages:         make(chan *logger.Message),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		successResult: &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: aws.String(nextSequenceToken),
		},
	}
	ticks := make(chan time.Time)
	newTicker = func(_ time.Duration) *time.Ticker {
		return &time.Ticker{
			C: ticks,
		}
	}

	go stream.collectBatch()

	stream.Log(&logger.Message{
		Line:      []byte(logline),
		Timestamp: time.Now(),
	})

	// Log an event 1 second later
	stream.Log(&logger.Message{
		Line:      []byte(logline),
		Timestamp: time.Now().Add(time.Second),
	})

	// Fire ticker in past to simulate negative event buffer age
	ticks <- time.Now().Add(-time.Second)

	// Verify single multiline event is flushed with a negative event buffer age
	argument := <-mockClient.putLogEventsArgument
	assert.Check(t, argument != nil, "Expected non-nil PutLogEventsInput")
	assert.Check(t, is.Equal(1, len(argument.LogEvents)), "Expected single multiline event")
	assert.Check(t, is.Equal(logline+"\n"+logline+"\n", *argument.LogEvents[0].Message), "Received incorrect multiline message")

	stream.Close()
}

func TestCollectBatchMultilinePatternMaxEventSize(t *testing.T) {
	mockClient := newMockClient()
	multilinePattern := regexp.MustCompile("xxxx")
	stream := &logStream{
		client:           mockClient,
		logGroupName:     groupName,
		logStreamName:    streamName,
		multilinePattern: multilinePattern,
		sequenceToken:    aws.String(sequenceToken),
		messages:         make(chan *logger.Message),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		successResult: &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: aws.String(nextSequenceToken),
		},
	}
	ticks := make(chan time.Time)
	newTicker = func(_ time.Duration) *time.Ticker {
		return &time.Ticker{
			C: ticks,
		}
	}

	go stream.collectBatch()

	// Log max event size
	longline := strings.Repeat("A", maximumBytesPerEvent)
	stream.Log(&logger.Message{
		Line:      []byte(longline),
		Timestamp: time.Now(),
	})

	// Log short event
	shortline := strings.Repeat("B", 100)
	stream.Log(&logger.Message{
		Line:      []byte(shortline),
		Timestamp: time.Now(),
	})

	// Fire ticker
	ticks <- time.Now().Add(batchPublishFrequency)

	// Verify multiline events
	// We expect a maximum sized event with no new line characters and a
	// second short event with a new line character at the end
	argument := <-mockClient.putLogEventsArgument
	assert.Check(t, argument != nil, "Expected non-nil PutLogEventsInput")
	assert.Check(t, is.Equal(2, len(argument.LogEvents)), "Expected two events")
	assert.Check(t, is.Equal(longline, *argument.LogEvents[0].Message), "Received incorrect multiline message")
	assert.Check(t, is.Equal(shortline+"\n", *argument.LogEvents[1].Message), "Received incorrect multiline message")
	stream.Close()
}

func TestCollectBatchClose(t *testing.T) {
	mockClient := newMockClient()
	stream := &logStream{
		client:        mockClient,
		logGroupName:  groupName,
		logStreamName: streamName,
		sequenceToken: aws.String(sequenceToken),
		messages:      make(chan *logger.Message),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		successResult: &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: aws.String(nextSequenceToken),
		},
	}
	var ticks = make(chan time.Time)
	newTicker = func(_ time.Duration) *time.Ticker {
		return &time.Ticker{
			C: ticks,
		}
	}

	go stream.collectBatch()

	stream.Log(&logger.Message{
		Line:      []byte(logline),
		Timestamp: time.Time{},
	})

	// no ticks
	stream.Close()

	argument := <-mockClient.putLogEventsArgument
	if argument == nil {
		t.Fatal("Expected non-nil PutLogEventsInput")
	}
	if len(argument.LogEvents) != 1 {
		t.Errorf("Expected LogEvents to contain 1 element, but contains %d", len(argument.LogEvents))
	}
	if *argument.LogEvents[0].Message != logline {
		t.Errorf("Expected message to be %s but was %s", logline, *argument.LogEvents[0].Message)
	}
}

func TestCollectBatchLineSplit(t *testing.T) {
	mockClient := newMockClient()
	stream := &logStream{
		client:        mockClient,
		logGroupName:  groupName,
		logStreamName: streamName,
		sequenceToken: aws.String(sequenceToken),
		messages:      make(chan *logger.Message),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		successResult: &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: aws.String(nextSequenceToken),
		},
	}
	var ticks = make(chan time.Time)
	newTicker = func(_ time.Duration) *time.Ticker {
		return &time.Ticker{
			C: ticks,
		}
	}

	go stream.collectBatch()

	longline := strings.Repeat("A", maximumBytesPerEvent)
	stream.Log(&logger.Message{
		Line:      []byte(longline + "B"),
		Timestamp: time.Time{},
	})

	// no ticks
	stream.Close()

	argument := <-mockClient.putLogEventsArgument
	if argument == nil {
		t.Fatal("Expected non-nil PutLogEventsInput")
	}
	if len(argument.LogEvents) != 2 {
		t.Errorf("Expected LogEvents to contain 2 elements, but contains %d", len(argument.LogEvents))
	}
	if *argument.LogEvents[0].Message != longline {
		t.Errorf("Expected message to be %s but was %s", longline, *argument.LogEvents[0].Message)
	}
	if *argument.LogEvents[1].Message != "B" {
		t.Errorf("Expected message to be %s but was %s", "B", *argument.LogEvents[1].Message)
	}
}

func TestCollectBatchMaxEvents(t *testing.T) {
	mockClient := newMockClientBuffered(1)
	stream := &logStream{
		client:        mockClient,
		logGroupName:  groupName,
		logStreamName: streamName,
		sequenceToken: aws.String(sequenceToken),
		messages:      make(chan *logger.Message),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		successResult: &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: aws.String(nextSequenceToken),
		},
	}
	var ticks = make(chan time.Time)
	newTicker = func(_ time.Duration) *time.Ticker {
		return &time.Ticker{
			C: ticks,
		}
	}

	go stream.collectBatch()

	line := "A"
	for i := 0; i <= maximumLogEventsPerPut; i++ {
		stream.Log(&logger.Message{
			Line:      []byte(line),
			Timestamp: time.Time{},
		})
	}

	// no ticks
	stream.Close()

	argument := <-mockClient.putLogEventsArgument
	if argument == nil {
		t.Fatal("Expected non-nil PutLogEventsInput")
	}
	if len(argument.LogEvents) != maximumLogEventsPerPut {
		t.Errorf("Expected LogEvents to contain %d elements, but contains %d", maximumLogEventsPerPut, len(argument.LogEvents))
	}

	argument = <-mockClient.putLogEventsArgument
	if argument == nil {
		t.Fatal("Expected non-nil PutLogEventsInput")
	}
	if len(argument.LogEvents) != 1 {
		t.Errorf("Expected LogEvents to contain %d elements, but contains %d", 1, len(argument.LogEvents))
	}
}

func TestCollectBatchMaxTotalBytes(t *testing.T) {
	expectedPuts := 2
	mockClient := newMockClientBuffered(expectedPuts)
	stream := &logStream{
		client:        mockClient,
		logGroupName:  groupName,
		logStreamName: streamName,
		sequenceToken: aws.String(sequenceToken),
		messages:      make(chan *logger.Message),
	}
	for i := 0; i < expectedPuts; i++ {
		mockClient.putLogEventsResult <- &putLogEventsResult{
			successResult: &cloudwatchlogs.PutLogEventsOutput{
				NextSequenceToken: aws.String(nextSequenceToken),
			},
		}
	}

	var ticks = make(chan time.Time)
	newTicker = func(_ time.Duration) *time.Ticker {
		return &time.Ticker{
			C: ticks,
		}
	}

	go stream.collectBatch()

	numPayloads := maximumBytesPerPut / (maximumBytesPerEvent + perEventBytes)
	// maxline is the maximum line that could be submitted after
	// accounting for its overhead.
	maxline := strings.Repeat("A", maximumBytesPerPut-(perEventBytes*numPayloads))
	// This will be split and batched up to the `maximumBytesPerPut'
	// (+/- `maximumBytesPerEvent'). This /should/ be aligned, but
	// should also tolerate an offset within that range.
	stream.Log(&logger.Message{
		Line:      []byte(maxline[:len(maxline)/2]),
		Timestamp: time.Time{},
	})
	stream.Log(&logger.Message{
		Line:      []byte(maxline[len(maxline)/2:]),
		Timestamp: time.Time{},
	})
	stream.Log(&logger.Message{
		Line:      []byte("B"),
		Timestamp: time.Time{},
	})

	// no ticks, guarantee batch by size (and chan close)
	stream.Close()

	argument := <-mockClient.putLogEventsArgument
	if argument == nil {
		t.Fatal("Expected non-nil PutLogEventsInput")
	}

	// Should total to the maximum allowed bytes.
	eventBytes := 0
	for _, event := range argument.LogEvents {
		eventBytes += len(*event.Message)
	}
	eventsOverhead := len(argument.LogEvents) * perEventBytes
	payloadTotal := eventBytes + eventsOverhead
	// lowestMaxBatch allows the payload to be offset if the messages
	// don't lend themselves to align with the maximum event size.
	lowestMaxBatch := maximumBytesPerPut - maximumBytesPerEvent

	if payloadTotal > maximumBytesPerPut {
		t.Errorf("Expected <= %d bytes but was %d", maximumBytesPerPut, payloadTotal)
	}
	if payloadTotal < lowestMaxBatch {
		t.Errorf("Batch to be no less than %d but was %d", lowestMaxBatch, payloadTotal)
	}

	argument = <-mockClient.putLogEventsArgument
	if len(argument.LogEvents) != 1 {
		t.Errorf("Expected LogEvents to contain 1 elements, but contains %d", len(argument.LogEvents))
	}
	message := *argument.LogEvents[len(argument.LogEvents)-1].Message
	if message[len(message)-1:] != "B" {
		t.Errorf("Expected message to be %s but was %s", "B", message[len(message)-1:])
	}
}

func TestCollectBatchWithDuplicateTimestamps(t *testing.T) {
	mockClient := newMockClient()
	stream := &logStream{
		client:        mockClient,
		logGroupName:  groupName,
		logStreamName: streamName,
		sequenceToken: aws.String(sequenceToken),
		messages:      make(chan *logger.Message),
	}
	mockClient.putLogEventsResult <- &putLogEventsResult{
		successResult: &cloudwatchlogs.PutLogEventsOutput{
			NextSequenceToken: aws.String(nextSequenceToken),
		},
	}
	ticks := make(chan time.Time)
	newTicker = func(_ time.Duration) *time.Ticker {
		return &time.Ticker{
			C: ticks,
		}
	}

	go stream.collectBatch()

	times := maximumLogEventsPerPut
	expectedEvents := []*cloudwatchlogs.InputLogEvent{}
	timestamp := time.Now()
	for i := 0; i < times; i++ {
		line := fmt.Sprintf("%d", i)
		if i%2 == 0 {
			timestamp.Add(1 * time.Nanosecond)
		}
		stream.Log(&logger.Message{
			Line:      []byte(line),
			Timestamp: timestamp,
		})
		expectedEvents = append(expectedEvents, &cloudwatchlogs.InputLogEvent{
			Message:   aws.String(line),
			Timestamp: aws.Int64(timestamp.UnixNano() / int64(time.Millisecond)),
		})
	}

	ticks <- time.Time{}
	stream.Close()

	argument := <-mockClient.putLogEventsArgument
	if argument == nil {
		t.Fatal("Expected non-nil PutLogEventsInput")
	}
	if len(argument.LogEvents) != times {
		t.Errorf("Expected LogEvents to contain %d elements, but contains %d", times, len(argument.LogEvents))
	}
	for i := 0; i < times; i++ {
		if !reflect.DeepEqual(*argument.LogEvents[i], *expectedEvents[i]) {
			t.Errorf("Expected event to be %v but was %v", *expectedEvents[i], *argument.LogEvents[i])
		}
	}
}

func TestParseLogOptionsMultilinePattern(t *testing.T) {
	info := logger.Info{
		Config: map[string]string{
			multilinePatternKey: "^xxxx",
		},
	}

	multilinePattern, err := parseMultilineOptions(info)
	assert.Check(t, err, "Received unexpected error")
	assert.Check(t, multilinePattern.MatchString("xxxx"), "No multiline pattern match found")
}

func TestParseLogOptionsDatetimeFormat(t *testing.T) {
	datetimeFormatTests := []struct {
		format string
		match  string
	}{
		{"%d/%m/%y %a %H:%M:%S%L %Z", "31/12/10 Mon 08:42:44.345 NZDT"},
		{"%Y-%m-%d %A %I:%M:%S.%f%p%z", "2007-12-04 Monday 08:42:44.123456AM+1200"},
		{"%b|%b|%b|%b|%b|%b|%b|%b|%b|%b|%b|%b", "Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec"},
		{"%B|%B|%B|%B|%B|%B|%B|%B|%B|%B|%B|%B", "January|February|March|April|May|June|July|August|September|October|November|December"},
		{"%A|%A|%A|%A|%A|%A|%A", "Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday"},
		{"%a|%a|%a|%a|%a|%a|%a", "Mon|Tue|Wed|Thu|Fri|Sat|Sun"},
		{"Day of the week: %w, Day of the year: %j", "Day of the week: 4, Day of the year: 091"},
	}
	for _, dt := range datetimeFormatTests {
		t.Run(dt.match, func(t *testing.T) {
			info := logger.Info{
				Config: map[string]string{
					datetimeFormatKey: dt.format,
				},
			}
			multilinePattern, err := parseMultilineOptions(info)
			assert.Check(t, err, "Received unexpected error")
			assert.Check(t, multilinePattern.MatchString(dt.match), "No multiline pattern match found")
		})
	}
}

func TestValidateLogOptionsDatetimeFormatAndMultilinePattern(t *testing.T) {
	cfg := map[string]string{
		multilinePatternKey: "^xxxx",
		datetimeFormatKey:   "%Y-%m-%d",
		logGroupKey:         groupName,
	}
	conflictingLogOptionsError := "you cannot configure log opt 'awslogs-datetime-format' and 'awslogs-multiline-pattern' at the same time"

	err := ValidateLogOpt(cfg)
	assert.Check(t, err != nil, "Expected an error")
	assert.Check(t, is.Equal(err.Error(), conflictingLogOptionsError), "Received invalid error")
}

func TestCreateTagSuccess(t *testing.T) {
	mockClient := newMockClient()
	info := logger.Info{
		ContainerName: "/test-container",
		ContainerID:   "container-abcdefghijklmnopqrstuvwxyz01234567890",
		Config:        map[string]string{"tag": "{{.Name}}/{{.FullID}}"},
	}
	logStreamName, e := loggerutils.ParseLogTag(info, loggerutils.DefaultTemplate)
	if e != nil {
		t.Errorf("Error generating tag: %q", e)
	}
	stream := &logStream{
		client:        mockClient,
		logGroupName:  groupName,
		logStreamName: logStreamName,
	}
	mockClient.createLogStreamResult <- &createLogStreamResult{}

	err := stream.create()

	if err != nil {
		t.Errorf("Received unexpected err: %v\n", err)
	}
	argument := <-mockClient.createLogStreamArgument

	if *argument.LogStreamName != "test-container/container-abcdefghijklmnopqrstuvwxyz01234567890" {
		t.Errorf("Expected LogStreamName to be %s", "test-container/container-abcdefghijklmnopqrstuvwxyz01234567890")
	}
}

func BenchmarkUnwrapEvents(b *testing.B) {
	events := make([]wrappedEvent, maximumLogEventsPerPut)
	for i := 0; i < maximumLogEventsPerPut; i++ {
		mes := strings.Repeat("0", maximumBytesPerEvent)
		events[i].inputLogEvent = &cloudwatchlogs.InputLogEvent{
			Message: &mes,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := unwrapEvents(events)
		assert.Check(b, is.Len(res, maximumLogEventsPerPut))
	}
}

func TestNewAWSLogsClientCredentialEndpointDetect(t *testing.T) {
	// required for the cloudwatchlogs client
	os.Setenv("AWS_REGION", "us-west-2")
	defer os.Unsetenv("AWS_REGION")

	credsResp := `{
		"AccessKeyId" :    "test-access-key-id",
		"SecretAccessKey": "test-secret-access-key"
		}`

	expectedAccessKeyID := "test-access-key-id"
	expectedSecretAccessKey := "test-secret-access-key"

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, credsResp)
	}))
	defer testServer.Close()

	// set the SDKEndpoint in the driver
	newSDKEndpoint = testServer.URL

	info := logger.Info{
		Config: map[string]string{},
	}

	info.Config["awslogs-credentials-endpoint"] = "/creds"

	c, err := newAWSLogsClient(info)
	assert.Check(t, err)

	client := c.(*cloudwatchlogs.CloudWatchLogs)

	creds, err := client.Config.Credentials.Get()
	assert.Check(t, err)

	assert.Check(t, is.Equal(expectedAccessKeyID, creds.AccessKeyID))
	assert.Check(t, is.Equal(expectedSecretAccessKey, creds.SecretAccessKey))
}

func TestNewAWSLogsClientCredentialEnvironmentVariable(t *testing.T) {
	// required for the cloudwatchlogs client
	os.Setenv("AWS_REGION", "us-west-2")
	defer os.Unsetenv("AWS_REGION")

	expectedAccessKeyID := "test-access-key-id"
	expectedSecretAccessKey := "test-secret-access-key"

	os.Setenv("AWS_ACCESS_KEY_ID", expectedAccessKeyID)
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")

	os.Setenv("AWS_SECRET_ACCESS_KEY", expectedSecretAccessKey)
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	info := logger.Info{
		Config: map[string]string{},
	}

	c, err := newAWSLogsClient(info)
	assert.Check(t, err)

	client := c.(*cloudwatchlogs.CloudWatchLogs)

	creds, err := client.Config.Credentials.Get()
	assert.Check(t, err)

	assert.Check(t, is.Equal(expectedAccessKeyID, creds.AccessKeyID))
	assert.Check(t, is.Equal(expectedSecretAccessKey, creds.SecretAccessKey))

}

func TestNewAWSLogsClientCredentialSharedFile(t *testing.T) {
	// required for the cloudwatchlogs client
	os.Setenv("AWS_REGION", "us-west-2")
	defer os.Unsetenv("AWS_REGION")

	expectedAccessKeyID := "test-access-key-id"
	expectedSecretAccessKey := "test-secret-access-key"

	contentStr := `
	[default]
	aws_access_key_id = "test-access-key-id"
	aws_secret_access_key =  "test-secret-access-key"
	`
	content := []byte(contentStr)

	tmpfile, err := ioutil.TempFile("", "example")
	defer os.Remove(tmpfile.Name()) // clean up
	assert.Check(t, err)

	_, err = tmpfile.Write(content)
	assert.Check(t, err)

	err = tmpfile.Close()
	assert.Check(t, err)

	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", tmpfile.Name())
	defer os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE")

	info := logger.Info{
		Config: map[string]string{},
	}

	c, err := newAWSLogsClient(info)
	assert.Check(t, err)

	client := c.(*cloudwatchlogs.CloudWatchLogs)

	creds, err := client.Config.Credentials.Get()
	assert.Check(t, err)

	assert.Check(t, is.Equal(expectedAccessKeyID, creds.AccessKeyID))
	assert.Check(t, is.Equal(expectedSecretAccessKey, creds.SecretAccessKey))
}
