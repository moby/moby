# CertToStore

[![Go Tests](https://github.com/google/certtostore/workflows/Go%20Tests/badge.svg)](https://github.com/google/certtostore/actions?query=workflow%3A%22Go+Tests%22)

CertToStore is a multi-platform package that allows you to work with x509
certificates on Linux and the certificate store on Windows.

## Why CertToStore?

CertToStore was created to solve some specific problems when working with
certificates using Go. Ever wanted to create public/private key pairs using the
TPM or create certificate requests using TPM backed keys? Both are possible
using CertToStore on Windows.

__Native Certificate Store Access without the prompts__ Certificate storage in
CertToStore under Windows is implemented using native Windows API calls. This
makes the package efficient and avoids problematic user prompts and
interactions.

With CertToStore, you can also lookup and use existing certificates with their
private keys through CNG, regardless of how they were issued (TPM or Software
backed).

__Built-in support for Cryptography API: Next Generation (CNG)__ CertToStore for
Windows was built from the ground up to use Microsoft's Cryptography API: Next
Generation (CNG). This grants certificates generated, requested, and stored
using CertToStore the ability to use your computer's TPM to store private key
material safely.

__Compatibile with packages that use x509.Certificate__ Certificates managed by
CertToStore are compatible with other packages that use
[x509.Certificate](https://golang.org/pkg/crypto/x509/). Want to generate
certificate requests using the TPM, and send them to your own third-party CA?
Have a Go based web server that you want to use with a TPM backed certificate?
Sure thing.

## Contact

We have a public discussion list at
[certtostore-discuss@googlegroups.com](https://groups.google.com/forum/#!forum/certtostore-discuss)

## Disclaimer

This is not an official Google product.
