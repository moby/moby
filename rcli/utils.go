package rcli

import (
	"github.com/dotcloud/docker/term"
	"os"
	"os/signal"
)

//FIXME: move these function to utils.go (in rcli to avoid import loop)
func SetRawTerminal() (*term.State, error) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		_ = <-c
		term.Restore(int(os.Stdin.Fd()), oldState)
		os.Exit(0)
	}()
	return oldState, err
}

func RestoreTerminal(state *term.State) {
	term.Restore(int(os.Stdin.Fd()), state)
}
