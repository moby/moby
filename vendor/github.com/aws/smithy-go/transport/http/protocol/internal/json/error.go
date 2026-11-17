package json

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// ProtocolErrorInfo holds the error type and message decoded from a JSON
// error response body.
type ProtocolErrorInfo struct {
	Type    string `json:"__type"`
	Message string

	// nonstandard, but some AWS services do present the type here
	Code any
}

// GetProtocolErrorInfo decodes error type/message from a JSON response body.
func GetProtocolErrorInfo(decoder *json.Decoder) (ProtocolErrorInfo, error) {
	var errInfo ProtocolErrorInfo
	if err := decoder.Decode(&errInfo); err != nil {
		if err == io.EOF {
			return errInfo, nil
		}
		return errInfo, err
	}
	return errInfo, nil
}

// ResolveProtocolErrorType resolves the error type from the header value and
// body info, returning the type and whether one was found.
func ResolveProtocolErrorType(headerType string, bodyInfo ProtocolErrorInfo) (string, bool) {
	if len(headerType) != 0 {
		return headerType, true
	} else if len(bodyInfo.Type) != 0 {
		return bodyInfo.Type, true
	} else if code, ok := bodyInfo.Code.(string); ok && len(code) != 0 {
		return code, true
	}
	return "", false
}

// SanitizeErrorCode strips namespace prefixes and URI suffixes from error
// codes received on the wire.
func SanitizeErrorCode(code string) string {
	if idx := strings.Index(code, ":"); idx != -1 {
		code = code[:idx]
	}
	if idx := strings.Index(code, "#"); idx != -1 {
		code = code[idx+1:]
	}
	return code
}

// EventStreamErrorInfo extracts the error code and message from a JSON
// exception payload.
func EventStreamErrorInfo(payload []byte) (string, string, error) {
	if len(payload) == 0 {
		return "", "", nil
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	info, err := GetProtocolErrorInfo(decoder)
	if err != nil {
		return "", "", err
	}

	var code string
	if typ, ok := ResolveProtocolErrorType("", info); ok {
		code = SanitizeErrorCode(typ)
	}
	return code, info.Message, nil
}
