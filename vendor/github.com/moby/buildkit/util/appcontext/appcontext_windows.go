package appcontext

import (
	"os"
)

var terminationSignals = []os.Signal{os.Interrupt}
