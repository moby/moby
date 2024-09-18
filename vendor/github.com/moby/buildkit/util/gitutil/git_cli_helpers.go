package gitutil

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func (cli *GitCLI) Dir() string {
	if cli.dir != "" {
		return cli.dir
	}
	return cli.workTree
}

func (cli *GitCLI) WorkTree(ctx context.Context) (string, error) {
	if cli.workTree != "" {
		return cli.workTree, nil
	}
	return cli.clean(cli.Run(ctx, "rev-parse", "--show-toplevel"))
}

func (cli *GitCLI) GitDir(ctx context.Context) (string, error) {
	if cli.gitDir != "" {
		return cli.gitDir, nil
	}

	dir, err := cli.WorkTree(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".git"), nil
}

func (cli *GitCLI) clean(dt []byte, err error) (string, error) {
	out := string(dt)
	out = strings.ReplaceAll(strings.Split(out, "\n")[0], "'", "")
	if err != nil {
		err = errors.New(strings.TrimSuffix(err.Error(), "\n"))
	}
	return out, err
}
