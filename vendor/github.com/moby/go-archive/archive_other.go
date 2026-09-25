//go:build !linux

package archive

func withProcSelfFD(opts *TarOptions) (*TarOptions, func(), error) {
	var prepared TarOptions
	if opts != nil {
		prepared = *opts
	}
	return &prepared, func() {}, nil
}

func getWhiteoutConverter(format WhiteoutFormat) tarWhiteoutConverter {
	return nil
}
