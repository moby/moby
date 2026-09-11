// Command namesgenerator wraps the name-generator fixture for the integration test.
package main

import (
	"github.com/moby/extensions/sdk"
	containernamegeneratorpb "github.com/moby/moby/v2/extpoints/containernamegenerator/v0/protogen"
	servicenamegeneratorpb "github.com/moby/moby/v2/extpoints/servicenamegenerator/v0/protogen"
	namesgeneratorimage "github.com/moby/moby/v2/internal/namesgenerator/image"
)

func main() {
	sdk.Main(namesgeneratorimage.Extension, sdk.WithServerPoints(
		containernamegeneratorpb.ServerPoint,
		servicenamegeneratorpb.ServerPoint,
	))
}
