package utils

import (
	"os"

	"github.com/Sirupsen/logrus"
)

// EnableDebug sets the DEBUG env var to true
// and makes the logger to log at debug level.
func EnableDebug() {
	os.Setenv("DEBUG", "1")
	logrus.SetLevel(logrus.DebugLevel)
}

// DisableDebug sets the DEBUG env var to false
// and makes the logger to log at info level.
func DisableDebug() {
	os.Setenv("DEBUG", "")
	logrus.SetLevel(logrus.InfoLevel)
}

// IsDebugEnabled checks whether the debug flag is set or not.
func IsDebugEnabled() bool {
	return os.Getenv("DEBUG") != ""
}
