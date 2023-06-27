# CHANGELOG

# v1.1.2

- [Improvement]: updated dependencies, test with Go 1.20.

# v1.1.1

- [Bug Fix]: fix test compilation on big endian machines.

# v1.1.0

**This is the first release of package packet that only supports Go 1.18+. Users
on older versions of Go must use v1.0.0.**

- [Improvement]: drop support for older versions of Go so we can begin using
  modern versions of `x/sys` and other dependencies.

## v1.0.0

**This is the last release of package vsock that supports Go 1.17 and below.**

- Initial stable commit! The API is mostly a direct translation of the previous
  `github.com/mdlayher/raw` package APIs, with some updates to make everything
  focused explicitly on Linux and `AF_PACKET` sockets. Functionally, the two
  packages are equivalent, and `*raw.Conn` is now backed by `*packet.Conn` in
  the latest version of the `raw` package.
