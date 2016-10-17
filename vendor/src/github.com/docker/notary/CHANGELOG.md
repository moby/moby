# Changelog

## [v0.4.2](https://github.com/docker/notary/releases/tag/v0.4.2) 9/30/2016
+ Bump the cross compiler to golang 1.7.1, since [1.6.3 builds binaries that could have non-deterministic bugs in OS X Sierra](https://groups.google.com/forum/#!msg/golang-dev/Jho5sBHZgAg/cq6d97S1AwAJ) [#984](https://github.com/docker/notary/pull/984)

## [v0.4.1](https://github.com/docker/notary/releases/tag/v0.4.1) 9/27/2016
+ Preliminary Windows support for notary client [#970](https://github.com/docker/notary/pull/970)
+ Output message to CLI when repo changes have been successfully published [#974](https://github.com/docker/notary/pull/974)
+ Improved error messages for client authentication errors and for the witness command [#972](https://github.com/docker/notary/pull/972)
+ Support for finding keys that are anywhere in the notary directory's "private" directory, not just under "private/root_keys" or "private/tuf_keys" [#981](https://github.com/docker/notary/pull/981)
+ Previously, on any error updating, the client would fall back on the cache.  Now we only do so if there is a network error or if the server is unavailable or missing the TUF data. Invalid TUF data will cause the update to fail - for example if there was an invalid root rotation. [#982](https://github.com/docker/notary/pull/982)

## [v0.4.0](https://github.com/docker/notary/releases/tag/v0.4.0) 9/21/2016
+ Server-managed key rotations [#889](https://github.com/docker/notary/pull/889)
+ Remove `timestamp_keys` table, which stored redundant information [#889](https://github.com/docker/notary/pull/889)
+ Introduce `notary delete` command to delete local and/or remote repo data [#895](https://github.com/docker/notary/pull/895)
+ Introduce `notary witness` command to stage signatures for specified roles [#875](https://github.com/docker/notary/pull/875)
+ Add `-p` flag to offline commands to attempt auto-publish [#886](https://github.com/docker/notary/pull/886) [#912](https://github.com/docker/notary/pull/912) [#923](https://github.com/docker/notary/pull/923)
+ Introduce `notary reset` command to manage staged changes [#959](https://github.com/docker/notary/pull/959) [#856](https://github.com/docker/notary/pull/856)
+ Add `--rootkey` flag to `notary init` to provide a private root key for a repo [#801](https://github.com/docker/notary/pull/801)
+ Introduce `notary delegation purge` command to remove a specified key from all delegations [#855](https://github.com/docker/notary/pull/855)
+ Removed HTTP endpoint from notary-signer [#870](https://github.com/docker/notary/pull/870)
+ Refactored and unified key storage [#825](https://github.com/docker/notary/pull/825)
+ Batched key import and export now operate on PEM files (potentially with multiple blocks) instead of ZIP [#825](https://github.com/docker/notary/pull/825) [#882](https://github.com/docker/notary/pull/882)
+ Add full database integration test-suite [#824](https://github.com/docker/notary/pull/824) [#854](https://github.com/docker/notary/pull/854) [#863](https://github.com/docker/notary/pull/863)
+ Improve notary-server, trust pinning, and yubikey logging [#798](https://github.com/docker/notary/pull/798) [#858](https://github.com/docker/notary/pull/858) [#891](https://github.com/docker/notary/pull/891)
+ Warn if certificates for root or delegations are near expiry [#802](https://github.com/docker/notary/pull/802)
+ Warn if role metadata is near expiry [#786](https://github.com/docker/notary/pull/786)
+ Reformat CLI table output to use the `text/tabwriter` package [#809](https://github.com/docker/notary/pull/809)
+ Fix passphrase retrieval attempt counting and terminal detection [#906](https://github.com/docker/notary/pull/906)
+ Fix listing nested delegations [#864](https://github.com/docker/notary/pull/864)
+ Bump go version to 1.6.3, fix go1.7 compatibility [#851](https://github.com/docker/notary/pull/851) [#793](https://github.com/docker/notary/pull/793)
+ Convert docker-compose files to v2 format [#755](https://github.com/docker/notary/pull/755)
+ Validate root rotations against trust pinning [#800](https://github.com/docker/notary/pull/800)
+ Update fixture certificates for two-year expiry window [#951](https://github.com/docker/notary/pull/951)

## [v0.3.0](https://github.com/docker/notary/releases/tag/v0.3.0) 5/11/2016
+ Root rotations
+ RethinkDB support as a storage backend for Server and Signer
+ A new TUF repo builder that merges server and client validation
+ Trust Pinning: configure known good key IDs and CAs to replace TOFU.
+ Add --input, --output, and --quiet flags to notary verify command
+ Remove local certificate store. It was redundant as all certs were also stored in the cached root.json
+ Cleanup of dead code in client side key storage logic
+ Update project to Go 1.6.1
+ Reorganize vendoring to meet Go 1.6+ standard. Still using Godeps to manage vendored packages
+ Add targets by hash, no longer necessary to have the original target data available
+ Active Key ID verification during signature verification
+ Switch all testing from assert to require, reduces noise in test runs
+ Use alpine based images for smaller downloads and faster setup times
+ Clean up out of data signatures when re-signing content
+ Set cache control headers on HTTP responses from Notary Server
+ Add sha512 support for targets
+ Add environment variable for delegation key passphrase
+ Reduce permissions requested by client from token server
+ Update formatting for delegation list output
+ Move SQLite dependency to tests only so it doesn't get built into official images
+ Fixed asking for password to list private repositories
+ Enable using notary client with username/password in a scripted fashion
+ Fix static compilation of client
+ Enforce TUF version to be >= 1, previously 0 was acceptable although unused
+ json.RawMessage should always be used as *json.RawMessage due to concepts of addressability in Go and effects on encoding

## [v0.2](https://github.com/docker/notary/releases/tag/v0.2.0) 2/24/2016
+ Add support for delegation roles in `notary` server and client
+ Add `notary CLI` commands for managing delegation roles: `notary delegation`
    + `add`, `list` and `remove` subcommands
+ Enhance `notary CLI` commands for adding targets to delegation roles
    + `notary add --roles` and `notary remove --roles` to manipulate targets for delegations
+ Support for rotating the snapshot key to one managed by the `notary` server
+ Add consistent download functionality to download metadata and content by checksum
+ Update `docker-compose` configuration to use official mariadb image
    + deprecate `notarymysql`
    + default to using a volume for `data` directory
    + use separate databases for `notary-server` and `notary-signer` with separate users
+ Add `notary CLI` command for changing private key passphrases: `notary key passwd`
+ Enhance `notary CLI` commands for importing and exporting keys
+ Change default `notary CLI` log level to fatal, introduce new verbose (error-level) and debug-level settings
+ Store roles as PEM headers in private keys, incompatible with previous notary v0.1 key format
    + No longer store keys as `<KEY_ID>_role.key`, instead store as `<KEY_ID>.key`; new private keys from new notary clients will crash old notary clients
+ Support logging as JSON format on server and signer
+ Support mutual TLS between notary client and notary server

## [v0.1](https://github.com/docker/notary/releases/tag/v0.1) 11/15/2015
+ Initial non-alpha `notary` version
+ Implement TUF (the update framework) with support for root, targets, snapshot, and timestamp roles
+ Add PKCS11 interface to store and sign with keys in HSMs (i.e. Yubikey)
