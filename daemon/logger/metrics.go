package logger // import "github.com/docker/docker/daemon/logger"

import (
	"github.com/docker/go-metrics"
)

var (
	logWritesFailedCount metrics.Counter
	logReadsFailedCount  metrics.Counter
	totalPartialLogs     metrics.Counter
)

func init() {
	loggerMetrics := metrics.NewNamespace("logger", "", nil)

	logWritesFailedCount = loggerMetrics.NewCounter("log_write_operations_failed", "Number of log write operations that failed")
	logReadsFailedCount = loggerMetrics.NewCounter("log_read_operations_failed", "Number of log reads from container stdio that failed")
	totalPartialLogs = loggerMetrics.NewCounter("log_entries_size_greater_than_buffer", "Number of log entries which are larger than the log buffer")

	metrics.Register(loggerMetrics)
}
