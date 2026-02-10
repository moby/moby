// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package serf

import (
	"github.com/hashicorp/memberlist"
)

type conflictDelegate struct {
	serf *Serf
}

func (c *conflictDelegate) NotifyConflict(existing, other *memberlist.Node) {
	c.serf.handleNodeConflict(existing, other)
}
