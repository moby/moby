package awslogs // import "github.com/docker/docker/daemon/logger/awslogs"

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

type mockClient struct {
	createLogGroupFunc  func(context.Context, *cloudwatchlogs.CreateLogGroupInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error)
	createLogStreamFunc func(context.Context, *cloudwatchlogs.CreateLogStreamInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error)
	putLogEventsFunc    func(context.Context, *cloudwatchlogs.PutLogEventsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error)
}

func (m *mockClient) CreateLogGroup(ctx context.Context, input *cloudwatchlogs.CreateLogGroupInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	return m.createLogGroupFunc(ctx, input, opts...)
}

func (m *mockClient) CreateLogStream(ctx context.Context, input *cloudwatchlogs.CreateLogStreamInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	return m.createLogStreamFunc(ctx, input, opts...)
}

func (m *mockClient) PutLogEvents(ctx context.Context, input *cloudwatchlogs.PutLogEventsInput, opts ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
	if err := checkPutLogEventsConstraints(input); err != nil {
		return nil, err
	}
	return m.putLogEventsFunc(ctx, input, opts...)
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

func (m *mockmetadataclient) GetRegion(context.Context, *imds.GetRegionInput, ...func(*imds.Options)) (*imds.GetRegionOutput, error) {
	output := <-m.regionResult
	err := output.errorResult
	if err != nil {
		return nil, err
	}
	return &imds.GetRegionOutput{Region: output.successResult}, err
}
