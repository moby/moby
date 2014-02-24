package namespaces

import (
	"github.com/dotcloud/docker/execdriver"
	"github.com/dotcloud/docker/pkg/term"
	"os"
)

type NsinitTerm struct {
	master *os.File
}

func NewTerm(pipes *execdriver.Pipes, master *os.File) *NsinitTerm {
	return &NsinitTerm{master}
}

func (t *NsinitTerm) Close() error {
	return t.master.Close()
}

func (t *NsinitTerm) Resize(h, w int) error {
	if t.master != nil {
		return term.SetWinsize(t.master.Fd(), &term.Winsize{Height: uint16(h), Width: uint16(w)})
	}
	return nil
}
