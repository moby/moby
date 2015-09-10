package awslogs

import "github.com/aws/aws-sdk-go/service/cloudwatchlogs"

type mockcwlogsclient struct {
	createLogStreamArgument chan *cloudwatchlogs.CreateLogStreamInput
	createLogStreamResult   chan *createLogStreamResult
	putLogEventsArgument    chan *cloudwatchlogs.PutLogEventsInput
	putLogEventsResult      chan *putLogEventsResult
}

type createLogStreamResult struct {
	successResult *cloudwatchlogs.CreateLogStreamOutput
	errorResult   error
}

type putLogEventsResult struct {
	successResult *cloudwatchlogs.PutLogEventsOutput
	errorResult   error
}

func newMockClient() *mockcwlogsclient {
	return &mockcwlogsclient{
		createLogStreamArgument: make(chan *cloudwatchlogs.CreateLogStreamInput, 1),
		createLogStreamResult:   make(chan *createLogStreamResult, 1),
		putLogEventsArgument:    make(chan *cloudwatchlogs.PutLogEventsInput, 1),
		putLogEventsResult:      make(chan *putLogEventsResult, 1),
	}
}

func newMockClientBuffered(buflen int) *mockcwlogsclient {
	return &mockcwlogsclient{
		createLogStreamArgument: make(chan *cloudwatchlogs.CreateLogStreamInput, buflen),
		createLogStreamResult:   make(chan *createLogStreamResult, buflen),
		putLogEventsArgument:    make(chan *cloudwatchlogs.PutLogEventsInput, buflen),
		putLogEventsResult:      make(chan *putLogEventsResult, buflen),
	}
}

func (m *mockcwlogsclient) CreateLogStream(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	m.createLogStreamArgument <- input
	output := <-m.createLogStreamResult
	return output.successResult, output.errorResult
}

func (m *mockcwlogsclient) PutLogEvents(input *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
	m.putLogEventsArgument <- input
	output := <-m.putLogEventsResult
	return output.successResult, output.errorResult
}

func test() {
	_ = &logStream{
		client: newMockClient(),
	}
}
