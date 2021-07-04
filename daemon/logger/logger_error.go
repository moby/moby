package logger

import (
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// Rates based on journald defaults of 10,000 messages in 30s.
// reference: https://www.freedesktop.org/software/systemd/man/journald.conf.html#RateLimitIntervalSec=
var logErrorLimiter = rate.NewLimiter(333, 333)

// logDriverError logs errors produced by log drivers to the daemon logs. It also increments the logWritesFailedCount
// metric.
// Logging to the daemon logs is limited to 333 operations per second at most. If this limit is exceeded, the
// logWritesFailedCount is still counted, but logging to the daemon logs is omitted in order to prevent disk saturation.
func logDriverError(loggerName, msgLine string, logErr error) {
	logWritesFailedCount.Inc(1)
	if logErrorLimiter.Allow() {
		logrus.WithError(logErr).
			WithField("driver", loggerName).
			WithField("message", msgLine).
			Errorf("Error writing log message")
	}
}
