// +build windows

package notary

import "os"

// NotarySupportedSignals does not contain any signals, because SIGUSR1/2 are not supported on windows
var NotarySupportedSignals = []os.Signal{}
