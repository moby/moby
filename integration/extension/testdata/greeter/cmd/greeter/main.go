// Command greeter wraps the published greeter fixture for the integration test.
package main

import (
	"github.com/moby/extensions/sdk"
	servicegrpcv0 "github.com/moby/moby/v2/extpoints/servicegrpc/v0"
	"github.com/moby/moby/v2/integration/extension/testdata/greeter"
)

func main() {
	sdk.Main(greeter.Extension, sdk.WithServerPoints(servicegrpcv0.ServerPoint))
}
