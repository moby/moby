// +build !dfrunmount,!dfextall

package dockerfile2llb

import (
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
)

func detectRunMount(cmd *command, dispatchStatesByName map[string]*dispatchState, allDispatchStates []*dispatchState) bool {
	return false
}

func dispatchRunMounts(d *dispatchState, c *instructions.RunCommand, sources []*dispatchState, opt dispatchOpt) []llb.RunOption {
	return nil
}
