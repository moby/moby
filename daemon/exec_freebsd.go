// +build freebsd

package daemon

// checkExecSupport returns an error if the exec driver does not support exec,
// or nil if it is supported.
func checkExecSupport(drivername string) error {
	return nil
}
