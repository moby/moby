package session

import "net/url"

func encodeHeaderValue(input string) (string, bool) {
	for _, r := range input {
		if r < 0x20 || r > 0x7e {
			return url.QueryEscape(input), true
		}
	}
	return input, false
}

func decodeHeaderValue(input string, encoded bool) string {
	if !encoded {
		return input
	}
	out, err := url.QueryUnescape(input)
	if err != nil {
		return input
	}
	return out
}
