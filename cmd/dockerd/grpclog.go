package main

import (
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/grpclog"
)

// grpc's default logger is *very* noisy and uses "info" and even "warn" level logging for mostly useless messages.
// This function configures the grpc logger to step down the severity of all messages.
//
// info => trace
// warn => debug
// error => warn
func configureGRPCLog() {
	l := logrus.WithField("library", "grpc")
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(l.WriterLevel(logrus.TraceLevel), l.WriterLevel(logrus.DebugLevel), l.WriterLevel(logrus.WarnLevel)))
}
