package daemon

import (
	// Importing packages here only to make sure their init gets called and
	// therefore they register themselves to the logdriver factory.
	_ "github.com/moby/moby/v2/daemon/logger/awslogs"
	_ "github.com/moby/moby/v2/daemon/logger/etwlogs"
	_ "github.com/moby/moby/v2/daemon/logger/fluentd"
	_ "github.com/moby/moby/v2/daemon/logger/gcplogs"
	_ "github.com/moby/moby/v2/daemon/logger/gelf"
	_ "github.com/moby/moby/v2/daemon/logger/jsonfilelog"
	_ "github.com/moby/moby/v2/daemon/logger/loggerutils/cache"
	_ "github.com/moby/moby/v2/daemon/logger/splunk"
	_ "github.com/moby/moby/v2/daemon/logger/syslog"
)
