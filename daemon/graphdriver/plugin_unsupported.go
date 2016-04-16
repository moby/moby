// +build !experimental

package graphdriver

func lookupPlugin(name string) (Bootstrap, error) {
	return nil, ErrNotSupported
}
