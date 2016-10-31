package log

import "google.golang.org/grpc/grpclog"

func init() {
	// completely replace the grpc logger with the logrus logger.
	grpclog.SetLogger(L)
}
