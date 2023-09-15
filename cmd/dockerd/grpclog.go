package main

import (
	"context"

	"github.com/containerd/containerd/log"
	"google.golang.org/grpc/grpclog"
)

// grpc's default logger is *very* noisy and uses "info" and even "warn" level logging for mostly useless messages.
// This function configures the grpc logger to step down the severity of all messages.
//
// info => trace
// warn => debug
// error => warn
func configureGRPCLog() {
	l := log.G(context.TODO()).WithField("library", "grpc")
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(l.WriterLevel(log.TraceLevel), l.WriterLevel(log.DebugLevel), l.WriterLevel(log.WarnLevel)))
}
