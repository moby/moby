// Copyright (c) 2021 PlanetScale Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package grpc

import (
	"github.com/planetscale/vtprotobuf/generator"
	"google.golang.org/protobuf/compiler/protogen"
)

const version = "1.1.0-vtproto"

var requireUnimplementedAlways = true
var requireUnimplemented = &requireUnimplementedAlways

func init() {
	generator.RegisterFeature("grpc", func(gen *generator.GeneratedFile) generator.FeatureGenerator {
		return &grpc{gen}
	})
}

type grpc struct {
	*generator.GeneratedFile
}

func (g *grpc) GenerateFile(file *protogen.File) bool {
	if len(file.Services) == 0 {
		return false
	}

	generateFileContent(nil, file, g.GeneratedFile)
	return true
}
