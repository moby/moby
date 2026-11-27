// Command offeredgreeter wraps the offered greeter fixture for the integration test.
package main

import (
	"github.com/moby/extensions/sdk"
	"github.com/moby/moby/v2/integration/extension/testdata/offeredgreeter"
)

func main() {
	sdk.Main(offeredgreeter.Extension, sdk.WithServerPoints(offeredgreeter.ServerPoint))
}
