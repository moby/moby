package signal

import (
	"os"
	"os/signal"
)

func CatchAll(sigc chan os.Signal) {
	handledSigs := []os.Signal{}
	for _, s := range signalMap {
		handledSigs = append(handledSigs, s)
	}
	signal.Notify(sigc, handledSigs...)
}

func StopCatch(sigc chan os.Signal) {
	signal.Stop(sigc)
	close(sigc)
}
