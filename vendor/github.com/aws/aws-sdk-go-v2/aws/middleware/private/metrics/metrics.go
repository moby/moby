// Package metrics implements metrics gathering for SDK development purposes.
//
// This package is designated as private and is intended for use only by the
// AWS client runtime. The exported API therein is not considered stable and
// is subject to breaking changes without notice.
package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aws/smithy-go/middleware"
)

const (
	// ServiceIDKey is the key for the service ID metric.
	ServiceIDKey = "ServiceId"
	// OperationNameKey is the key for the operation name metric.
	OperationNameKey = "OperationName"
	// ClientRequestIDKey is the key for the client request ID metric.
	ClientRequestIDKey = "ClientRequestId"
	// APICallDurationKey is the key for the API call duration metric.
	APICallDurationKey = "ApiCallDuration"
	// APICallSuccessfulKey is the key for the API call successful metric.
	APICallSuccessfulKey = "ApiCallSuccessful"
	// MarshallingDurationKey is the key for the marshalling duration metric.
	MarshallingDurationKey = "MarshallingDuration"
	// InThroughputKey is the key for the input throughput metric.
	InThroughputKey = "InThroughput"
	// OutThroughputKey is the key for the output throughput metric.
	OutThroughputKey = "OutThroughput"
	// RetryCountKey is the key for the retry count metric.
	RetryCountKey = "RetryCount"
	// HTTPStatusCodeKey is the key for the HTTP status code metric.
	HTTPStatusCodeKey = "HttpStatusCode"
	// AWSExtendedRequestIDKey is the key for the AWS extended request ID metric.
	AWSExtendedRequestIDKey = "AwsExtendedRequestId"
	// AWSRequestIDKey is the key for the AWS request ID metric.
	AWSRequestIDKey = "AwsRequestId"
	// BackoffDelayDurationKey is the key for the backoff delay duration metric.
	BackoffDelayDurationKey = "BackoffDelayDuration"
	// StreamThroughputKey is the key for the stream throughput metric.
	StreamThroughputKey = "Throughput"
	// ConcurrencyAcquireDurationKey is the key for the concurrency acquire duration metric.
	ConcurrencyAcquireDurationKey = "ConcurrencyAcquireDuration"
	// PendingConcurrencyAcquiresKey is the key for the pending concurrency acquires metric.
	PendingConcurrencyAcquiresKey = "PendingConcurrencyAcquires"
	// SigningDurationKey is the key for the signing duration metric.
	SigningDurationKey = "SigningDuration"
	// UnmarshallingDurationKey is the key for the unmarshalling duration metric.
	UnmarshallingDurationKey = "UnmarshallingDuration"
	// TimeToFirstByteKey is the key for the time to first byte metric.
	TimeToFirstByteKey = "TimeToFirstByte"
	// ServiceCallDurationKey is the key for the service call duration metric.
	ServiceCallDurationKey = "ServiceCallDuration"
	// EndpointResolutionDurationKey is the key for the endpoint resolution duration metric.
	EndpointResolutionDurationKey = "EndpointResolutionDuration"
	// AttemptNumberKey is the key for the attempt number metric.
	AttemptNumberKey = "AttemptNumber"
	// MaxConcurrencyKey is the key for the max concurrency metric.
	MaxConcurrencyKey = "MaxConcurrency"
	// AvailableConcurrencyKey is the key for the available concurrency metric.
	AvailableConcurrencyKey = "AvailableConcurrency"
)

// MetricPublisher provides the interface to provide custom MetricPublishers.
// PostRequestMetrics will be invoked by the MetricCollection middleware to post request.
// PostStreamMetrics will be invoked by ReadCloserWithMetrics to post stream metrics.
type MetricPublisher interface {
	PostRequestMetrics(*MetricData) error
	PostStreamMetrics(*MetricData) error
}

// Serializer provides the interface to provide custom Serializers.
// Serialize will transform any input object in its corresponding string representation.
type Serializer interface {
	Serialize(obj interface{}) (string, error)
}

// DefaultSerializer is an implementation of the Serializer interface.
type DefaultSerializer struct{}

// Serialize uses the default JSON serializer to obtain the string representation of an object.
func (DefaultSerializer) Serialize(obj interface{}) (string, error) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

type metricContextKey struct{}

// MetricContext contains fields to store metric-related information.
type MetricContext struct {
	connectionCounter *SharedConnectionCounter
	publisher         MetricPublisher
	data              *MetricData
}

// MetricData stores the collected metric data.
type MetricData struct {
	RequestStartTime           time.Time
	RequestEndTime             time.Time
	APICallDuration            time.Duration
	SerializeStartTime         time.Time
	SerializeEndTime           time.Time
	MarshallingDuration        time.Duration
	ResolveEndpointStartTime   time.Time
	ResolveEndpointEndTime     time.Time
	EndpointResolutionDuration time.Duration
	InThroughput               float64
	OutThroughput              float64
	RetryCount                 int
	Success                    uint8
	StatusCode                 int
	ClientRequestID            string
	ServiceID                  string
	OperationName              string
	PartitionID                string
	Region                     string
	RequestContentLength       int64
	Stream                     StreamMetrics
	Attempts                   []AttemptMetrics
}

// StreamMetrics stores metrics related to streaming data.
type StreamMetrics struct {
	ReadDuration time.Duration
	ReadBytes    int64
	Throughput   float64
}

// AttemptMetrics stores metrics related to individual attempts.
type AttemptMetrics struct {
	ServiceCallStart           time.Time
	ServiceCallEnd             time.Time
	ServiceCallDuration        time.Duration
	FirstByteTime              time.Time
	TimeToFirstByte            time.Duration
	ConnRequestedTime          time.Time
	ConnObtainedTime           time.Time
	ConcurrencyAcquireDuration time.Duration
	CredentialFetchStartTime   time.Time
	CredentialFetchEndTime     time.Time
	SignStartTime              time.Time
	SignEndTime                time.Time
	SigningDuration            time.Duration
	DeserializeStartTime       time.Time
	DeserializeEndTime         time.Time
	UnMarshallingDuration      time.Duration
	RetryDelay                 time.Duration
	ResponseContentLength      int64
	StatusCode                 int
	RequestID                  string
	ExtendedRequestID          string
	HTTPClient                 string
	MaxConcurrency             int
	PendingConnectionAcquires  int
	AvailableConcurrency       int
	ActiveRequests             int
	ReusedConnection           bool
}

// Data returns the MetricData associated with the MetricContext.
func (mc *MetricContext) Data() *MetricData {
	return mc.data
}

// ConnectionCounter returns the SharedConnectionCounter associated with the MetricContext.
func (mc *MetricContext) ConnectionCounter() *SharedConnectionCounter {
	return mc.connectionCounter
}

// Publisher returns the MetricPublisher associated with the MetricContext.
func (mc *MetricContext) Publisher() MetricPublisher {
	return mc.publisher
}

// ComputeRequestMetrics calculates and populates derived metrics based on the collected data.
func (md *MetricData) ComputeRequestMetrics() {

	for idx := range md.Attempts {
		attempt := &md.Attempts[idx]
		attempt.ConcurrencyAcquireDuration = attempt.ConnObtainedTime.Sub(attempt.ConnRequestedTime)
		attempt.SigningDuration = attempt.SignEndTime.Sub(attempt.SignStartTime)
		attempt.UnMarshallingDuration = attempt.DeserializeEndTime.Sub(attempt.DeserializeStartTime)
		attempt.TimeToFirstByte = attempt.FirstByteTime.Sub(attempt.ServiceCallStart)
		attempt.ServiceCallDuration = attempt.ServiceCallEnd.Sub(attempt.ServiceCallStart)
	}

	md.APICallDuration = md.RequestEndTime.Sub(md.RequestStartTime)
	md.MarshallingDuration = md.SerializeEndTime.Sub(md.SerializeStartTime)
	md.EndpointResolutionDuration = md.ResolveEndpointEndTime.Sub(md.ResolveEndpointStartTime)

	md.RetryCount = len(md.Attempts) - 1

	latestAttempt, err := md.LatestAttempt()

	if err != nil {
		fmt.Printf("error retrieving attempts data due to: %s. Skipping Throughput metrics", err.Error())
	} else {

		md.StatusCode = latestAttempt.StatusCode

		if md.Success == 1 {
			if latestAttempt.ResponseContentLength > 0 && latestAttempt.ServiceCallDuration > 0 {
				md.InThroughput = float64(latestAttempt.ResponseContentLength) / latestAttempt.ServiceCallDuration.Seconds()
			}
			if md.RequestContentLength > 0 && latestAttempt.ServiceCallDuration > 0 {
				md.OutThroughput = float64(md.RequestContentLength) / latestAttempt.ServiceCallDuration.Seconds()
			}
		}
	}
}

// LatestAttempt returns the latest attempt metrics.
// It returns an error if no attempts are initialized.
func (md *MetricData) LatestAttempt() (*AttemptMetrics, error) {
	if md.Attempts == nil || len(md.Attempts) == 0 {
		return nil, fmt.Errorf("no attempts initialized. NewAttempt() should be called first")
	}
	return &md.Attempts[len(md.Attempts)-1], nil
}

// NewAttempt initializes new attempt metrics.
func (md *MetricData) NewAttempt() {
	if md.Attempts == nil {
		md.Attempts = []AttemptMetrics{}
	}
	md.Attempts = append(md.Attempts, AttemptMetrics{})
}

// SharedConnectionCounter is a counter shared across API calls.
type SharedConnectionCounter struct {
	mu sync.Mutex

	activeRequests           int
	pendingConnectionAcquire int
}

// ActiveRequests returns the count of active requests.
func (cc *SharedConnectionCounter) ActiveRequests() int {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	return cc.activeRequests
}

// PendingConnectionAcquire returns the count of pending connection acquires.
func (cc *SharedConnectionCounter) PendingConnectionAcquire() int {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	return cc.pendingConnectionAcquire
}

// AddActiveRequest increments the count of active requests.
func (cc *SharedConnectionCounter) AddActiveRequest() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.activeRequests++
}

// RemoveActiveRequest decrements the count of active requests.
func (cc *SharedConnectionCounter) RemoveActiveRequest() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.activeRequests--
}

// AddPendingConnectionAcquire increments the count of pending connection acquires.
func (cc *SharedConnectionCounter) AddPendingConnectionAcquire() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.pendingConnectionAcquire++
}

// RemovePendingConnectionAcquire decrements the count of pending connection acquires.
func (cc *SharedConnectionCounter) RemovePendingConnectionAcquire() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.pendingConnectionAcquire--
}

// InitMetricContext initializes the metric context with the provided counter and publisher.
// It returns the updated context.
func InitMetricContext(
	ctx context.Context, counter *SharedConnectionCounter, publisher MetricPublisher,
) context.Context {
	if middleware.GetStackValue(ctx, metricContextKey{}) == nil {
		ctx = middleware.WithStackValue(ctx, metricContextKey{}, &MetricContext{
			connectionCounter: counter,
			publisher:         publisher,
			data: &MetricData{
				Attempts: []AttemptMetrics{},
				Stream:   StreamMetrics{},
			},
		})
	}
	return ctx
}

// Context returns the metric context from the given context.
// It returns nil if the metric context is not found.
func Context(ctx context.Context) *MetricContext {
	mctx := middleware.GetStackValue(ctx, metricContextKey{})
	if mctx == nil {
		return nil
	}
	return mctx.(*MetricContext)
}
