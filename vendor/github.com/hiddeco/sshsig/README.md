# sshsig

[![Go Reference](https://pkg.go.dev/badge/github.com/openssh/sshsig.svg)][godoc]
[![Go Report Card](https://goreportcard.com/badge/github.com/hiddeco/sshsig)][goreport]

This Go library implements the [`SSHSIG` wire protocol][sshsig-protocol], and
can be used to sign and verify messages using SSH keys.

Compared to other implementations, this library does all the following:

- Accepts an `io.Reader` as input for signing and verifying messages.
- Performs simple public key fingerprint and namespace mismatch checks in
  `Verify`. Malicious input will still fail signature verification, but this
  provides more useful error messages.
- Properly uses `ssh-sha2-512` as signature algorithm when signing with an RSA
  private key, as [described in the protocol][sshsig-rsa-req].
- Does not accept a `Sign` operation without a `namespace` as [specified in the
  protocol][sshsig-namespace-req].
- Allows `Verify` operations to be performed without a `namespace`, ensuring
  compatibility with loose implementations.
- Provides `Armor` and `Unarmor` functions to encode/decode the signature
  to/from an (armored) PEM format.

For more information about the use of this library, see the [Go Reference][godoc].

## Acknowledgements

There are several other implementations of the `SSHSIG` protocol in Go, from
which this library has borrowed ideas:

- [go-sshsig][go-sshsig] by Paul Tagliamonte
- [Sigstore Rekor][rekor-ssh] from the Sigstore project

[sshsig-protocol]: https://github.com/openssh/openssh-portable/blob/V_9_2_P1/PROTOCOL.sshsig
[sshsig-rsa-req]: https://github.com/openssh/openssh-portable/blob/V_9_2_P1/PROTOCOL.sshsig#L69-L72
[sshsig-namespace-req]: https://github.com/openssh/openssh-portable/blob/V_9_2_P1/PROTOCOL.sshsig#L57
[go-sshsig]: https://github.com/paultag/go-sshsig/tree/a684343203bd83859fbe5783fc976948b4413010
[rekor-ssh]: https://github.com/sigstore/rekor/tree/v1.0.1/pkg/pki/ssh
[godoc]: https://pkg.go.dev/github.com/hiddeco/sshsig
[goreport]: https://goreportcard.com/report/github.com/hiddeco/sshsig
