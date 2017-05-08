# Roadmap

The Trust project consists of a number of moving parts of which Notary Server is one. Notary Server is the front line metadata service
that clients interact with. It manages TUF metadata and interacts with a pluggable signing service to issue new TUF timestamp
files.

The Notary-signer is provided as our reference implementation of a signing service. It supports HSMs along with Ed25519 software signing.
