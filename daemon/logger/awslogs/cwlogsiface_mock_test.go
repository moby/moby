package awslogs // import "github.com/docker/docker/daemon/logger/awslogs"

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

type mockClient struct {
	createLogGroupFunc  func(input *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error)
	createLogStreamFunc func(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error)
	putLogEventsFunc    func(input *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error)
}

func (m *mockClient) CreateLogGroup(input *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	return m.createLogGroupFunc(input)
}

func (m *mockClient) CreateLogStream(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	return m.createLogStreamFunc(input)
}

func (m *mockClient) PutLogEvents(input *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
	if err := checkPutLogEventsConstraints(input); err != nil {
		return nil, err
	}
	return m.putLogEventsFunc(input)
}

func checkPutLogEventsConstraints(input *cloudwatchlogs.PutLogEventsInput) error {
	events := input.LogEvents
	// Checked enforced limits in mock
	totalBytes := 0
	for _, evt := range events {
		if evt.Message == nil {
			continue
		}
		eventBytes := len([]byte(*evt.Message))
		if eventBytes > maximumBytesPerEvent {
			// exceeded per event message size limits
			return fmt.Errorf("maximum bytes per event exceeded: Event too large %d, max allowed: %d", eventBytes, maximumBytesPerEvent)
		}
		// total event bytes including overhead
		totalBytes += eventBytes + perEventBytes
	}

	if totalBytes > maximumBytesPerPut {
		// exceeded per put maximum size limit
		return fmt.Errorf("maximum bytes per put exceeded: Upload too large %d, max allowed: %d", totalBytes, maximumBytesPerPut)
	}
	return nil
}

type mockmetadataclient struct {
	regionResult chan *regionResult
}

type regionResult struct {
	successResult string
	errorResult   error
}

func newMockMetadataClient() *mockmetadataclient {
	return &mockmetadataclient{
		regionResult: make(chan *regionResult, 1),
	}
}

func (m *mockmetadataclient) Region() (string, error) {
	output := <-m.regionResult
	return output.successResult, output.errorResult
}
