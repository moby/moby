package evaluator

import (
	"fmt"
	"strings"
)

func env(b *buildFile, args ...string) error {
	if len(args) != 2 {
		return fmt.Errorf("ENV accepts two arguments")
	}

	// the duplication here is intended to ease the replaceEnv() call's env
	// handling. This routine gets much shorter with the denormalization here.
	key := args[0]
	b.env[key] = args[1]
	b.config.Env = append(b.config.Env, strings.Join("=", key, b.env[key]))

	return b.commit("", b.config.Cmd, fmt.Sprintf("ENV %s", value))
}

func maintainer(b *buildFile, args ...string) error {
	if len(args) != 1 {
		return fmt.Errorf("MAINTAINER requires only one argument")
	}

	b.maintainer = args[0]
	return b.commit("", b.config.Cmd, fmt.Sprintf("MAINTAINER %s", b.maintainer))
}

func add(b *buildFile, args ...string) error {
	if len(args) != 2 {
		return fmt.Errorf("ADD requires two arguments")
	}

	return b.runContextCommand(args, true, true, "ADD")
}

func dispatchCopy(b *buildFile, args ...string) error {
	if len(args) != 2 {
		return fmt.Errorf("COPY requires two arguments")
	}

	return b.runContextCommand(args, false, false, "COPY")
}
