//go:build !cgo || static_build || !libnftables

package nftables

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/internal/rootless"
	"go.opentelemetry.io/otel"
)

type nftCtx struct{}

var lookPathNSEnter = sync.OnceValues(func() (string, error) {
	return exec.LookPath("nsenter")
})
var lookPathNft = sync.OnceValues(func() (string, error) {
	p, err := exec.LookPath("nft")
	if err != nil {
		log.G(context.Background()).WithError(err).Warnf("Failed to find nft tool")
		return "", fmt.Errorf("failed to find nft tool: %w", err)
	}
	return p, nil
})

func preflight() error {
	_, err := lookPathNft()
	return err
}

func newNftCtx() (*nftCtx, error) {
	_, err := lookPathNft()
	if err != nil {
		return nil, err
	}
	return &nftCtx{}, nil
}

func (*nftCtx) Apply(ctx context.Context, nftCmd []byte) error {
	ctx, span := otel.Tracer("").Start(ctx, spanPrefix+".nftApply.exec")
	defer span.End()

	cmdPath, err := lookPathNft()
	if err != nil {
		return err
	}
	cmdArgs := []string{cmdPath, "-f", "-"}
	detachedNetNS, err := rootless.DetachedNetNS()
	if err != nil {
		return fmt.Errorf("could not check for detached netns: %w", err)
	}
	if detachedNetNS != "" && !rootless.InSandboxNS() {
		nsenterPath, err := lookPathNSEnter()
		if err != nil {
			return fmt.Errorf("nsenter not found: %w", err)
		}
		cmdPath = nsenterPath
		cmdArgs = append([]string{nsenterPath, "-n" + detachedNetNS, "-F", "--"}, cmdArgs...)
	}
	cmd := exec.CommandContext(ctx, cmdPath, cmdArgs[1:]...)
	stdoutBuf := strings.Builder{}
	stderrBuf := strings.Builder{}
	cmd.Stdin = bytes.NewReader(nftCmd)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running nft: %s %w", stderrBuf.String(), err)
	}
	log.G(ctx).WithFields(log.Fields{"stdout": stdoutBuf.String(), "stderr": stderrBuf.String()}).Debug("nftables: updated")
	return nil
}

func (*nftCtx) Close() {
}
