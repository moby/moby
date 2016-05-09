// Copyright 2014 Google Inc.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uuid

import "errors"

func (u UUID) MarshalJSON() ([]byte, error) {
	if len(u) != 16 {
		return []byte(`""`), nil
	}
	var js [38]byte
	js[0] = '"'
	encodeHex(js[1:], u)
	js[37] = '"'
	return js[:], nil
}

func (u *UUID) UnmarshalJSON(data []byte) error {
	if string(data) == `""` {
		return nil
	}
	if data[0] != '"' {
		return errors.New("invalid UUID format")
	}
	data = data[1 : len(data)-1]
	uu := Parse(string(data))
	if uu == nil {
		return errors.New("invalid UUID format")
	}
	*u = uu
	return nil
}
