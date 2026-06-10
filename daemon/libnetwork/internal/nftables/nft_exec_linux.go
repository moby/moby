//go:build !cgo || static_build || no_libnftables

package nftables

import (
	"context"
	"fmt"
	"io"
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
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("getting stdin pipe for nft: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("getting stdout pipe for nft: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("getting stderr pipe for nft: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting nft: %w", err)
	}
	if _, err := stdinPipe.Write(nftCmd); err != nil {
		return fmt.Errorf("sending nft commands: %w", err)
	}
	if err := stdinPipe.Close(); err != nil {
		return fmt.Errorf("closing nft input pipe: %w", err)
	}

	stdoutBuf := strings.Builder{}
	if _, err := io.Copy(&stdoutBuf, stdoutPipe); err != nil {
		return fmt.Errorf("reading stdout of nft: %w", err)
	}
	stdout := stdoutBuf.String()
	stderrBuf := strings.Builder{}
	if _, err := io.Copy(&stderrBuf, stderrPipe); err != nil {
		return fmt.Errorf("reading stderr of nft: %w", err)
	}
	stderr := stderrBuf.String()

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("running nft: %s %w", stderr, err)
	}
	log.G(ctx).WithFields(log.Fields{"stdout": stdout, "stderr": stderr}).Debug("nftables: updated")
	return nil
}

func (*nftCtx) Close() {
}
