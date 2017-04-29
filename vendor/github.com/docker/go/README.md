[![Circle CI](https://circleci.com/gh/jfrazelle/go.svg?style=svg)](https://circleci.com/gh/jfrazelle/go)

This is a repository used for building go packages based off upstream with
small patches.

It is only used so far for a canonical json pkg.

I hope we do not need to use it for anything else in the future.

**To update:**

```console
$ make update
```

This will nuke the current package, clone upstream and apply the patch.
