package daemon

import (
	// Importing packages here only to make sure their init gets called and
	// therefore they register themselves to the logdriver factory.
	_ "github.com/moby/moby/v2/daemon/logger/jsonfilelog"
	_ "github.com/moby/moby/v2/daemon/logger/local"
	_ "github.com/moby/moby/v2/daemon/logger/loggerutils/cache"
)
