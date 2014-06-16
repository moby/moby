package term

import (
	"fmt"
	"strings"
)

var ASCII = []string{
	"ctrl-@",
	"ctrl-a",
	"ctrl-b",
	"ctrl-c",
	"ctrl-d",
	"ctrl-e",
	"ctrl-f",
	"ctrl-g",
	"ctrl-h",
	"ctrl-i",
	"ctrl-j",
	"ctrl-k",
	"ctrl-l",
	"ctrl-m",
	"ctrl-n",
	"ctrl-o",
	"ctrl-p",
	"ctrl-q",
	"ctrl-r",
	"ctrl-s",
	"ctrl-t",
	"ctrl-u",
	"ctrl-v",
	"ctrl-w",
	"ctrl-x",
	"ctrl-y",
	"ctrl-z",
	"ctrl-[",
	"ctrl-\\",
	"ctrl-]",
	"ctrl-^",
	"ctrl-_",
}

func ToBytes(keys string) ([]byte, error) {
	codes := []byte{}
next:
	for _, key := range strings.Split(keys, ",") {
		if len(key) != 1 {
			for code, ctrl := range ASCII {
				if ctrl == key {
					codes = append(codes, byte(code))
					continue next
				}
			}
			return nil, fmt.Errorf("Unknown character: '%s'", key)
		} else {
			codes = append(codes, byte(key[0]))
		}
	}
	return codes, nil
}
