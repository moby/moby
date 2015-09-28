package errors

// This file contains all of the errors that can be generated from the
// daemon/logger component.

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeLogAWSInvOption is generated when specifed options are invalid.
	ErrorCodeLogAWSInvOption = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGAWSINVOPTION",
		Message:        "unknown log opt '%s' for %s log driver",
		Description:    "Indicates that the options specified to the logger are invalid.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogAWSNoOptionVal is generated when the value for a log option is missing or empty.
	ErrorCodeLogAWSNoOptionVal = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGAWSNOOPTIONVAL",
		Message:        "must specify a value for log opt '%s'",
		Description:    "Indicates that the value for a log option is missing or empty.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogAWSNoRgnKey is generated when AWS region key is not specified in the log option or as an env variable.
	ErrorCodeLogAWSNoRgnKey = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGAWSNORGNKEY",
		Message:        "must specify a value for environment variable '%s' or log opt '%s'",
		Description:    "Indicates that the AWS region key is not specified in the log options or as env variable.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogErrResolveHostname is generated when logger fails to resolve the hostname of the host.
	ErrorCodeLogErrResolveHostname = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGERRRESOLVEHOSTNAME",
		Message:        "logger: can not resolve hostname: %v",
		Description:    "Indicates that logger fails to resolve the hostname of the host.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogFailedToWrite is generated when logger failed to log a message.
	ErrorCodeLogFailedToWrite = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGFAILEDTOWRITE",
		Message:        "Failed to log msg %q for logger %s: %s",
		Description:    "Indicates that the logger failed to log a message.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogErrScan is generated when logger fails to scan the logstream.
	ErrorCodeLogErrScan = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGERRSCAN",
		Message:        "Error scanning log stream: %s",
		Description:    "Indicates that the logger fails to scan the logstream.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogDriverAlreadyReg is generated when log driver with the name already registered.
	ErrorCodeLogDriverAlreadyReg = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGDRIVERALREADYREG",
		Message:        "logger: log driver named '%s' is already registered",
		Description:    "Indicates that the log driver with the name is already registered",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogValAlreadyReg is generated when the log validator with the name is already registered.
	ErrorCodeLogValAlreadyReg = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGVALALREADYREG",
		Message:        "logger: log validator named '%s' is already registered",
		Description:    "Indicates that the log validator with the name is already registered.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogErrNoDriverReg is generated when the log driver with the name is not registered.
	ErrorCodeLogErrNoDriverReg = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGERRNODRIVERREG",
		Message:        "logger: no log driver named '%s' is registered",
		Description:    "Indicates that the the log driver with the name is not registered.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogFluentdErrOpt is generated when a invalid option is specified to fluentd log driver.
	ErrorCodeLogFluentdErrOpt = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGFLUENTDERROPT",
		Message:        "unknown log opt '%s' for fluentd log driver",
		Description:    "Indicates that the option is invalid to fluentd log driver.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogFluentdInvAddr is generated address specified to the fluentd log driver is invalid.
	ErrorCodeLogFluentdInvAddr = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGFLUENTDINVADDR",
		Message:        "invalid fluentd-address %s: %s",
		Description:    "Indicates that the address specified to the fluentd log driver is invalid.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogGelfErrHostNameAccess is generated when hostname cannot be accessed.
	ErrorCodeLogGelfErrHostNameAccess = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGGELFERRHOSTNAMEACCESS",
		Message:        "gelf: cannot access hostname to set source field",
		Description:    "Indicates that the gelf logger cannot access the hostname.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogGelfInvEndpoint is generated when invalid endpoint is used to initialize gelf logger.
	ErrorCodeLogGelfInvEndpoint = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGGELFINVENDPOINT",
		Message:        "gelf: cannot connect to GELF endpoint: %s %v",
		Description:    "Indicates that a invalid endpoint is used to initialize gelf logger.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogGelfFailedToWrite is generated when gelf logger failed to write a log message.
	ErrorCodeLogGelfFailedToWrite = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGGELFFAILEDTOWRITE",
		Message:        "gelf: cannot send GELF message: %v",
		Description:    "Indicates that the gel logger failed to write a log message.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogGelfErrOpt is generated when a invalid option is specified to gelf log driver.
	ErrorCodeLogGelfErrOpt = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGGELFERROPT",
		Message:        "unknown log opt '%s' for gelf log driver",
		Description:    "Indicates that a invalid option is specified to gelf log driver. ",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogGelfInvAddress is generated when the address is incorrectly formmatted for the gelf driver.
	ErrorCodeLogGelfInvAddress = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGGELFINVADDRESS",
		Message:        "gelf-address should be in form proto://address, got %v",
		Description:    "Indicates that the address is incorrectly formmatted for the gelf driver.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogGelfErrNotUDP is generated when the gelf endpoint is not UDP.
	ErrorCodeLogGelfErrNotUDP = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGGELFERRNOTUDP",
		Message:        "gelf: endpoint needs to be UDP",
		Description:    "Indicates that the gelf endpoint is not UDP",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogGelfInvHostPort is generated when the Host and Port are incorrectly specified in the gelf address.
	ErrorCodeLogGelfInvHostPort = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGGELFINVHOSTPORT",
		Message:        "gelf: please provide gelf-address as udp://host:port",
		Description:    "Indicates that the Host and Port are incorrectly specified in the gelf address.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogJDNotEnabled is generated when journald is not enabled on the host.
	ErrorCodeLogJDNotEnabled = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGJDNOTENABLED",
		Message:        "journald is not enabled on this host",
		Description:    "Indicates that journald is not enabled on the host.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogJDErrOpt is generated when a invalid option is specified to journald log driver.
	ErrorCodeLogJDErrOpt = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGJDERROPT",
		Message:        "unknown log opt '%s' for journald log driver",
		Description:    "Indicates that a invalid option is specified to journald log driver.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogJDErrOpen is generated when an error occurred while opening journal.
	ErrorCodeLogJDErrOpen = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGJDERROPEN",
		Message:        "error opening journal",
		Description:    "Indicates that an error occurred when opening journal.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogJDErrSetThreshold is generated when journald fails to set journal data threshold.
	ErrorCodeLogJDErrSetThreshold = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGJDERRSETTHRESHOLD",
		Message:        "error setting journal data threshold",
		Description:    "Indicates that the journald fails to set journal data threshold",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogJDErrSetMatch is generated when journald fails to set a match for search.
	ErrorCodeLogJDErrSetMatch = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGJDERRSETMATCH",
		Message:        "error setting journal match",
		Description:    "Indicates that journald fails to set a match for search.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogJDErrSeekEnd is generated when error occurred seeking to the end of the journal.
	ErrorCodeLogJDErrSeekEnd = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGJDERRSEEKEND",
		Message:        "error seeking to end of journal",
		Description:    "Indicates that an error occurred seeking to the end of the journal.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogJDErrSeekPrev is generated when an error occurred backtraking to previous journal entry.
	ErrorCodeLogJDErrSeekPrev = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGJDERRSEEKPREV",
		Message:        "error backtracking to previous journal entry",
		Description:    "Indicates that an error occurred backtraking to previous journal entry.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogJDErrSeekStart is generated when error occurred seeking to the start of the journal.
	ErrorCodeLogJDErrSeekStart = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGJDERRSEEKSTART",
		Message:        "error seeking to start of journal",
		Description:    "Indicates that an error occurred seeking to the start of the journal.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogJDErrSeekStartTime is generated when error occurred seeking to the start time of the journal.
	ErrorCodeLogJDErrSeekStartTime = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGJDERRSEEKSTARTTIME",
		Message:        "error seeking to start time in journa",
		Description:    "Indicates that an error occurred seeking to the start time of the journal.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogJDErrSeekNext is generated when an error occurred seeking to next journal entry.
	ErrorCodeLogJDErrSeekNext = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGJDERRSEEKNEXT",
		Message:        "error skipping to next journal entry",
		Description:    "Indicates that an error occurred seeking to next journal entry.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogJDErrOpenPipe is generated when an error occurred opening a pipe for notifications.
	ErrorCodeLogJDErrOpenPipe = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGJDERROPENPIPE",
		Message:        "error opening journald close notification pipe",
		Description:    "Indicates that an error occurred opening a pipe for notifications.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogJSONErrFileSize is generated when max files option for json-file logger is set to less than 1.
	ErrorCodeLogJSONErrFileSize = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGJSONERRFILESIZE",
		Message:        "max-file cannot be less than 1",
		Description:    "Indicates that max files option for json-file logger is set to less than 1",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogJSONErrOpt is generated when a invalid option is specified to json-file log driver.
	ErrorCodeLogJSONErrOpt = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGJSONERROPT",
		Message:        "unknown log opt '%s' for json-file log driver",
		Description:    "Indicates that a invalid option is specified to json-file log driver.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogSysInvAddress is generated when the address is incorrectly formatted for syslog driver.
	ErrorCodeLogSysInvAddress = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGSYSINVADDRESS",
		Message:        "syslog-address should be in form proto://address, got %v",
		Description:    "Indicates that the address is incorrectly formatted for the syslog driver.",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLogSysErrOpt is generated when a invalid option is specified to syslog driver.
	ErrorCodeLogSysErrOpt = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOGSYSERROPT",
		Message:        "unknown log opt '%s' for syslog log driver",
		Description:    "Indicates that a invalid option is specified to syslog driver.",
		HTTPStatusCode: http.StatusInternalServerError,
	})
)
