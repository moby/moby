# go-pathspec

[![build](https://github.com/shibumi/go-pathspec/workflows/build/badge.svg)](https://github.com/shibumi/go-pathspec/actions?query=workflow%3Abuild) [![Coverage Status](https://coveralls.io/repos/github/shibumi/go-pathspec/badge.svg)](https://coveralls.io/github/shibumi/go-pathspec) [![PkgGoDev](https://pkg.go.dev/badge/github.com/shibumi/go-pathspec)](https://pkg.go.dev/github.com/shibumi/go-pathspec)

go-pathspec implements gitignore-style pattern matching for paths.

## Alternatives

There are a few alternatives, that try to be gitignore compatible or even state
gitignore compatibility:

### https://github.com/go-git/go-git

go-git states it would be gitignore compatible, but actually they are missing a few
special cases. This issue describes one of the not working patterns: https://github.com/go-git/go-git/issues/108

What does not work is global filename pattern matching. Consider the following
`.gitignore` file:

```gitignore
# gitignore test file
parse.go
```

Then `parse.go` should match on all filenames called `parse.go`. You can test this via
this shell script:
```shell
mkdir -p /tmp/test/internal/util
touch /tmp/test/internal/util/parse.go
cd /tmp/test/
git init
echo "parse.go" > .gitignore
```

With git `parse.go` will be excluded. The go-git implementation behaves different.

### https://github.com/monochromegane/go-gitignore

monochromegane's go-gitignore does not support the use of `**`-operators.
This is not consistent to real gitignore behavior, too.

## Authors

Sander van Harmelen (<sander@xanzy.io>)  
Christian Rebischke (<chris@shibumi.dev>)
