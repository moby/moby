package utils

import (
	"os"
	"os/signal"
)

func StopCatch(sigc chan os.Signal) {
	signal.Stop(sigc)
	close(sigc)
}
