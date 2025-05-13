package utils // import "github.com/docker/docker/distribution/utils"

import (
	"io"

	gometrics "github.com/docker/go-metrics"
)

var (
	metricsNS = gometrics.NewNamespace("engine", "daemon", nil)

	// ImagePullBytes is a running tally of the bytes downloaded and extracted
	// by image pull operations. By taking the derrivative of this metric over
	// time, image pull throughput can be measured.
	ImagePullBytes = metricsNS.NewCounter("image_pull_bytes", "The number of bytes processed as part of image pulls")
)

// MetricsReader is a reader which keeps track of the number of bytes read in
// the provided Counter.
type MetricsReader struct {
	io.ReadCloser
	Counter gometrics.Counter
}

// Read increments the Counter by the number of bytes read
func (m *MetricsReader) Read(p []byte) (int, error) {
	read, err := m.ReadCloser.Read(p)
	// we are counting bytes here. This is OK; the number of bytes may be very
	// large, but the counter is uint64, which is enough for exabytes of data.
	m.Counter.Inc(float64(read))
	return read, err
}
