package data

import (
	"fmt"
	"strconv"
	"strings"
)

func Encode(obj map[string][]string) string {
	var msg string
	msg += encodeHeader(0)
	for k, values := range obj {
		msg += encodeNamedList(k, values)
	}
	return msg
}

func encodeHeader(msgtype int) string {
	return fmt.Sprintf("%03.3d;", msgtype)
}

func encodeString(s string) string {
	return fmt.Sprintf("%d:%s,", len(s), s)
}

var EncodeString = encodeString
var DecodeString = decodeString

func encodeList(l []string) string {
	values := make([]string, 0, len(l))
	for _, s := range l {
		values = append(values, encodeString(s))
	}
	return encodeString(strings.Join(values, ""))
}

func encodeNamedList(name string, l []string) string {
	return encodeString(name) + encodeList(l)
}

func Decode(msg string) (map[string][]string, error) {
	msgtype, skip, err := decodeHeader(msg)
	if err != nil {
		return nil, err
	}
	if msgtype != 0 {
		// FIXME: use special error type so the caller can easily ignore
		return nil, fmt.Errorf("unknown message type: %d", msgtype)
	}
	msg = msg[skip:]
	obj := make(map[string][]string)
	for len(msg) > 0 {
		k, skip, err := decodeString(msg)
		if err != nil {
			return nil, err
		}
		msg = msg[skip:]
		values, skip, err := decodeList(msg)
		if err != nil {
			return nil, err
		}
		msg = msg[skip:]
		obj[k] = values
	}
	return obj, nil
}

func decodeList(msg string) ([]string, int, error) {
	blob, skip, err := decodeString(msg)
	if err != nil {
		return nil, 0, err
	}
	var l []string
	for len(blob) > 0 {
		v, skipv, err := decodeString(blob)
		if err != nil {
			return nil, 0, err
		}
		l = append(l, v)
		blob = blob[skipv:]
	}
	return l, skip, nil
}

func decodeString(msg string) (string, int, error) {
	parts := strings.SplitN(msg, ":", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid format: no column")
	}
	var length int
	if l, err := strconv.ParseUint(parts[0], 10, 64); err != nil {
		return "", 0, err
	} else {
		length = int(l)
	}
	if len(parts[1]) < length+1 {
		return "", 0, fmt.Errorf("message '%s' is %d bytes, expected at least %d", parts[1], len(parts[1]), length+1)
	}
	payload := parts[1][:length+1]
	if payload[length] != ',' {
		return "", 0, fmt.Errorf("message is not comma-terminated")
	}
	return payload[:length], len(parts[0]) + 1 + length + 1, nil
}

func decodeHeader(msg string) (int, int, error) {
	if len(msg) < 4 {
		return 0, 0, fmt.Errorf("message too small")
	}
	msgtype, err := strconv.ParseInt(msg[:3], 10, 32)
	if err != nil {
		return 0, 0, err
	}
	return int(msgtype), 4, nil
}
