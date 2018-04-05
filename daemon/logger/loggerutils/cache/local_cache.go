package cache // import "github.com/docker/docker/daemon/logger/loggerutils/cache"

import (
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/local"
	"github.com/sirupsen/logrus"
)

// WithLocalCache wraps the passed in logger with a logger caches all writes locally
// in addition to writing to the passed in logger.
func WithLocalCache(l logger.Logger, logInfo logger.Info) (logger.Logger, error) {
	localLogger, err := local.New(logInfo)
	if err != nil {
		return nil, err
	}
	return &loggerWithCache{
		l: l,
		// TODO(@cpuguy83): Should this be configurable?
		cache: logger.NewRingLogger(localLogger, logInfo, -1),
	}, nil
}

type loggerWithCache struct {
	l     logger.Logger
	cache logger.Logger
}

func (l *loggerWithCache) Log(msg *logger.Message) error {
	// copy the message since the underlying logger will return the passed in message to the message pool
	dup := logger.NewMessage()
	dumbCopyMessage(dup, msg)
	if err := l.l.Log(msg); err != nil {
		return err
	}
	return l.cache.Log(dup)
}

func (l *loggerWithCache) Name() string {
	return l.l.Name()
}

func (l *loggerWithCache) ReadLogs(config logger.ReadConfig) *logger.LogWatcher {
	return l.cache.(logger.LogReader).ReadLogs(config)
}

func (l *loggerWithCache) Close() error {
	err := l.l.Close()
	if err := l.cache.Close(); err != nil {
		logrus.WithError(err).Warn("error while shutting cache logger")
	}
	return err
}

// dumbCopyMessage is a bit of a fake copy but avoids extra allocations which
// are not necessary for this use case.
func dumbCopyMessage(dst, src *logger.Message) {
	dst.Source = src.Source
	dst.Timestamp = src.Timestamp
	dst.PLogMetaData = src.PLogMetaData
	dst.Err = src.Err
	dst.Attrs = src.Attrs
	dst.Line = src.Line
}
