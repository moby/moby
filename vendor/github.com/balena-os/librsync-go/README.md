# librsync-go

librsync-go is a reimplementation of [librsync](https://github.com/librsync/librsync) in Go.

## Installing

To install the rdiff utility:

```sh
go install github.com/balena-os/librsync-go/cmd/rdiff
```

To use it as a library simply include `github.com/balena-os/librsync-go` in your import statement

## Contributing

If you're interested in contributing, that's awesome!

### Pull requests

Here's a few guidelines to make the process easier for everyone involved.

- We use [Versionist](https://github.com/product-os/versionist) to manage
  versioning (and in particular, [semantic versioning](https://semver.org)) and
  generate the changelog for this project.
- At least one commit in a PR should have a `Change-Type: type` footer, where
  `type` can be `patch`, `minor` or `major`. The subject of this commit will be
  added to the changelog.
- Commits should be squashed as much as makes sense.
- Commits should be signed-off (`git commit -s`)
