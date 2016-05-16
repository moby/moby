// +build !experimental

package storage

func lookupPlugin(name, home string, opts []string) (Driver, error) {
	return nil, ErrNotSupported
}
