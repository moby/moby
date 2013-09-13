package utils

import (
	"mime"
)

func MatchesContentType(contentType, expectedType string) bool {
	mimetype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		Debugf("Error parsing media type: %s error: %s", contentType, err.Error())
	}
	return err == nil && mimetype == expectedType
}
