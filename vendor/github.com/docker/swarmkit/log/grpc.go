package log

import (
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/grpclog"
)

type logrusWrapper struct {
	*logrus.Entry
}

// V provides the functionality that returns whether a particular log level is at
// least l - this is needed to meet the LoggerV2 interface.  GRPC's logging levels
// are: https://github.com/grpc/grpc-go/blob/master/grpclog/loggerv2.go#L71
// 0=info, 1=warning, 2=error, 3=fatal
// logrus's are: https://github.com/sirupsen/logrus/blob/master/logrus.go
// 0=panic, 1=fatal, 2=error, 3=warn, 4=info, 5=debug
func (lw logrusWrapper) V(l int) bool {
	// translate to logrus level
	logrusLevel := 4 - l
	return int(lw.Logger.Level) <= logrusLevel
}

func init() {
	ctx := WithModule(context.Background(), "grpc")

	// completely replace the grpc logger with the logrus logger.
	grpclog.SetLoggerV2(logrusWrapper{Entry: G(ctx)})
}
