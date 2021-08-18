// +build dfheredoc

package parser

import "github.com/moby/buildkit/frontend/dockerfile/command"

func init() {
	heredocDirectives = map[string]bool{
		command.Add:  true,
		command.Copy: true,
		command.Run:  true,
	}

	heredocCompoundDirectives = map[string]bool{
		command.Onbuild: true,
	}
}
