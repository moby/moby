package xml

import "encoding/xml"

type protocolError struct {
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// GetProtocolErrorInfo extracts Smithy error details from a query-protocol
// response.
//
// This version of GetProtocolErrorInfo also handles the extraction of the
// modeled error body from the protocol wrapper so the caller can operate on it
// directly.
func GetProtocolErrorInfo(payload []byte) (code, message string, errorBody []byte, err error) {
	code = "UnknownError"
	message = code

	errorBody, err = ExtractElement(payload, "Error")
	if err != nil || len(errorBody) == 0 {
		return
	}

	var errinf protocolError
	if err = xml.Unmarshal(errorBody, &errinf); err != nil {
		return
	}

	err = nil
	if len(errinf.Code) > 0 {
		code = errinf.Code
	}
	if len(errinf.Message) > 0 {
		message = errinf.Message
	}
	return
}
