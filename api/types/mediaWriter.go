package types

import "io"

// MediaWriter is a Writer for an explicit Media type. MediaType can be used by producer to adjust content
type mediaWriter struct {
	io.Writer
	mediaType string
}

// WithMediaType return Writer and implements HasMediaType
func WithMediaType(w io.Writer, mediaType string) io.Writer {
	return mediaWriter{
		Writer:    w,
		mediaType: mediaType,
	}
}

func (w mediaWriter) MediaType() string {
	return w.mediaType
}

type HasMediaType interface {
	MediaType() string
}
