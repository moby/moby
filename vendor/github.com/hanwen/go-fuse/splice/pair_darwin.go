// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package splice

import ()

func (p *Pair) LoadFromAt(fd uintptr, sz int, off int64) (int, error) {
	panic("not implemented")
	return 0, nil
}

func (p *Pair) LoadFrom(fd uintptr, sz int) (int, error) {
	panic("not implemented")
	return 0, nil
}

func (p *Pair) WriteTo(fd uintptr, n int) (int, error) {
	panic("not implemented")
	return 0, nil
}
