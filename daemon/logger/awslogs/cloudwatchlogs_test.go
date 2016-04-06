package awslogs

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/dockerversion"
)

const (
	groupName         = "groupName"
	streamName        = "streamName"
	sequenceToken     = "sequenceToken"
	nextSequenceToken = "nextSequenceToken"
	logline           = "this is a log line"
)

func TestNewAWSLogsClientUserAgentHandler(t *testing.T) {
	ctx := logger.Context{
		Config: map[string]string{
			regionKey: "us-east-1",
		},
	}

	client, err := newAWSLogsClient(ctx)
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
	expectedUserAgentString := fmt.Sprintf("Docker %s (%s) %s/%s",
		dockerversion.Version, runtime.GOOS, aws.SDKName, aws.SDKVersion)
	userAgent := request.HTTPRequest.Header.Get("User-Agent")
	if userAgent != expectedUserAgentString {
		t.Errorf("Wrong User-Agent string, expected \"%s\" but was \"%s\"",
			expectedUserAgentString, userAgent)
	}
}

func TestNewAWSLogsClientRegionDetect(t *testing.T) {
	ctx := logger.Context{
		Config: map[string]string{},
	}

	mockMetadata := newMockMetadataClient()
	newRegionFinder = func() regionFinder {
		return mockMetadata
	}
	mockMetadata.regionResult <- &regionResult{
		successResult: "us-east-1",
	}

	_, err := newAWSLogsClient(ctx)
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
		t.Fatal("Expected non-nil LogGroupName")
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
		errorResult: errors.New("Error!"),
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

	events := []*cloudwatchlogs.InputLogEvent{
		{
			Message: aws.String(logline),
		},
	}

	stream.publishBatch(events)
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
	if argument.LogEvents[0] != events[0] {
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
		errorResult: errors.New("Error!"),
	}

	events := []*cloudwatchlogs.InputLogEvent{
		{
			Message: aws.String(logline),
		},
	}

	stream.publishBatch(events)
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

	events := []*cloudwatchlogs.InputLogEvent{
		{
			Message: aws.String(logline),
		},
	}

	stream.publishBatch(events)
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
	if argument.LogEvents[0] != events[0] {
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
	if argument.LogEvents[0] != events[0] {
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

	events := []*cloudwatchlogs.InputLogEvent{
		{
			Message: aws.String(logline),
		},
	}

	stream.publishBatch(events)
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
	if argument.LogEvents[0] != events[0] {
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

	longline := strings.Repeat("A", maximumBytesPerPut)
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
	bytes := 0
	for _, event := range argument.LogEvents {
		bytes += len(*event.Message)
	}
	if bytes > maximumBytesPerPut {
		t.Errorf("Expected <= %d bytes but was %d", maximumBytesPerPut, bytes)
	}

	argument = <-mockClient.putLogEventsArgument
	if len(argument.LogEvents) != 1 {
		t.Errorf("Expected LogEvents to contain 1 elements, but contains %d", len(argument.LogEvents))
	}
	message := *argument.LogEvents[0].Message
	if message[len(message)-1:] != "B" {
		t.Errorf("Expected message to be %s but was %s", "B", message[len(message)-1:])
	}
}
