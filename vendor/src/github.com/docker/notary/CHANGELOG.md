# Changelog

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
