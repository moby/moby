//go:build tools
// +build tools

// Package tools tracks dependencies on binaries not referenced in this codebase.
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
// Disclaimer: Avoid adding tools that don't need to be inferred from go.mod
// like golangci-lint and check they don't import too many dependencies.
package tools

import (
	_ "github.com/gogo/protobuf/protoc-gen-gogo"
	_ "github.com/gogo/protobuf/protoc-gen-gogofast"
	_ "github.com/gogo/protobuf/protoc-gen-gogoslick"
	_ "github.com/golang/protobuf/protoc-gen-go"
)
