// +build !linux,!windows

package fsutil

import (
	"runtime"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func Receive(ctx context.Context, conn Stream, dest string, notifyHashed ChangeFunc) error {
	return errors.Errorf("receive is unsupported in %s", runtime.GOOS)
}
