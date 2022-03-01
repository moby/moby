package streamformatter

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

// MediaType implements HasMediaType
func (w mediaWriter) MediaType() string {
	return w.mediaType
}

// HasMediaType is for types which declare an explicit Media Type
type HasMediaType interface {
	MediaType() string
}

// MediaTypeJSONSequence is media type for RFC-7464
const MediaTypeJSONSequence = "application/json-seq"
