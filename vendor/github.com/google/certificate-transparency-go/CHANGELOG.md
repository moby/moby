# CERTIFICATE-TRANSPARENCY-GO Changelog

## HEAD

## v1.1.7

* Recommended Go version for development: 1.20
  * This is the version used by the Cloud Build presubmits. Using a different version can lead to presubmits failing due to unexpected diffs.

* Bump golangci-lint from 1.51.1 to 1.55.1 (developers should update to this version).

### Add support for WASI port

* Add build tags for wasip1 GOOS by @flavio in https://github.com/google/certificate-transparency-go/pull/1089

### Add support for IBM Z operating system z/OS

* Add build tags for zOS by @onlywork1984 in https://github.com/google/certificate-transparency-go/pull/1088

### Log List

* Add support for "is_all_logs" field in loglist3 by @phbnf in https://github.com/google/certificate-transparency-go/pull/1095

### Documentation

* Improve Dockerized Test Deployment documentation by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1179

### Misc

* Escape forward slashes in certificate Subject names when used as user quota id strings by @robstradling in https://github.com/google/certificate-transparency-go/pull/1059
* Search whole chain looking for issuer match by @mhutchinson in https://github.com/google/certificate-transparency-go/pull/1112
* Use proper check per @AGWA instead of buggy check introduced in #1112 by @mhutchinson in https://github.com/google/certificate-transparency-go/pull/1114
* Build the ctfe/ct_server binary without depending on glibc by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1119
* Migrate CTFE Ingress manifest to support GKE version 1.23 by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1086
* Remove Dependabot ignore configuration by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1097
* Add "github-actions" and "docker" Dependabot config by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1101
* Add top level permission in CodeQL workflow by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1102
* Pin Docker image dependencies by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1110
* Remove GO111MODULE from Dockerfile and Cloud Build yaml files by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1113
* Add docker Dependabot config by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1126
* Export is_mirror = 0.0 for non mirror instead of nothing by @phbnf in https://github.com/google/certificate-transparency-go/pull/1133
* Add govulncheck GitHub action by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1145
* Spelling by @jsoref in https://github.com/google/certificate-transparency-go/pull/1144

### Dependency update

* Bump Go from 1.19 to 1.20 by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1146
* Bump golangci-lint from 1.51.1 to 1.55.1 by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1214
* Bump go.etcd.io/etcd/v3 from 3.5.8 to 3.5.9 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1083
* Bump golang.org/x/crypto from 0.8.0 to 0.9.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/108
* Bump github.com/mattn/go-sqlite3 from 1.14.16 to 1.14.17 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1092
* Bump golang.org/x/net from 0.10.0 to 0.11.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1094
* Bump github.com/prometheus/client_golang from 1.15.1 to 1.16.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1098
* Bump google.golang.org/protobuf from 1.30.0 to 1.31.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1099
* Bump golang.org/x/net from 0.11.0 to 0.12.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1108
* Bump actions/checkout from 3.1.0 to 3.5.3 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1103
* Bump github/codeql-action from 2.1.27 to 2.20.3 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1104
* Bump ossf/scorecard-action from 2.0.6 to 2.2.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1105
* Bump actions/upload-artifact from 3.1.0 to 3.1.2 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1106
* Bump github/codeql-action from 2.20.3 to 2.20.4 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1115
* Bump github/codeql-action from 2.20.4 to 2.21.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1117
* Bump golang.org/x/net from 0.12.0 to 0.14.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1124
* Bump github/codeql-action from 2.21.0 to 2.21.2 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1121
* Bump github/codeql-action from 2.21.2 to 2.21.4 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1125
* Bump golang from `fd9306e` to `eb3f9ac` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1127
* Bump alpine from 3.8 to 3.18 in /trillian/examples/deployment/docker/envsubst by @dependabot in https://github.com/google/certificate-transparency-go/pull/1129
* Bump golang from `fd9306e` to `eb3f9ac` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1128
* Bump alpine from `82d1e9d` to `7144f7b` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1130
* Bump golang from `fd9306e` to `eb3f9ac` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1131
* Bump golang from 1.19-alpine to 1.21-alpine in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1132
* Bump actions/checkout from 3.5.3 to 3.6.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1134
* Bump github/codeql-action from 2.21.4 to 2.21.5 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1135
* Bump distroless/base from `73deaaf` to `46c5b9b` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1136
* Bump actions/checkout from 3.6.0 to 4.0.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1137
* Bump golang.org/x/net from 0.14.0 to 0.15.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1139
* Bump github.com/rs/cors from 1.9.0 to 1.10.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1140
* Bump actions/upload-artifact from 3.1.2 to 3.1.3 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1141
* Bump golang from `445f340` to `96634e5` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1142
* Bump github/codeql-action from 2.21.5 to 2.21.6 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1149
* Bump Docker golang base images to 1.21.1 by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1147
* Bump github/codeql-action from 2.21.6 to 2.21.7 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1150
* Bump github/codeql-action from 2.21.7 to 2.21.8 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1152
* Bump golang from `d3114db` to `a0b3bc4` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1155
* Bump golang from `d3114db` to `a0b3bc4` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1157
* Bump golang from `d3114db` to `a0b3bc4` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1156
* Bump golang from `d3114db` to `a0b3bc4` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1158
* Bump golang from `e06b3a4` to `114b9cc` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1159
* Bump golang from `a0b3bc4` to `114b9cc` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1160
* Bump golang from `a0b3bc4` to `114b9cc` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1161
* Bump actions/checkout from 4.0.0 to 4.1.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1162
* Bump golang from `114b9cc` to `9c7ea4a` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1163
* Bump golang from `114b9cc` to `9c7ea4a` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1166
* Bump golang from `114b9cc` to `9c7ea4a` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1165
* Bump golang from `114b9cc` to `9c7ea4a` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1164
* Bump github/codeql-action from 2.21.8 to 2.21.9 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1169
* Bump golang from `9c7ea4a` to `61f84bc` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1168
* Bump github.com/prometheus/client_golang from 1.16.0 to 1.17.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1172
* Bump golang from `9c7ea4a` to `61f84bc` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1170
* Bump github.com/rs/cors from 1.10.0 to 1.10.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1176
* Bump alpine from `7144f7b` to `eece025` in /trillian/examples/deployment/docker/envsubst by @dependabot in https://github.com/google/certificate-transparency-go/pull/1174
* Bump alpine from `7144f7b` to `eece025` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1175
* Bump golang from `9c7ea4a` to `61f84bc` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1171
* Bump golang from `9c7ea4a` to `61f84bc` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1173
* Bump distroless/base from `46c5b9b` to `a35b652` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1177
* Bump golang.org/x/crypto from 0.13.0 to 0.14.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1178
* Bump github/codeql-action from 2.21.9 to 2.22.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1180
* Bump golang from 1.21.1-bookworm to 1.21.2-bookworm in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1181
* Bump golang.org/x/net from 0.15.0 to 0.16.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1184
* Bump golang from 1.21.1-bookworm to 1.21.2-bookworm in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1182
* Bump golang from 1.21.1-bookworm to 1.21.2-bookworm in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1185
* Bump golang from 1.21.1-bookworm to 1.21.2-bookworm in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1183
* Bump github/codeql-action from 2.22.0 to 2.22.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1186
* Bump distroless/base from `a35b652` to `b31a6e0` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1188
* Bump ossf/scorecard-action from 2.2.0 to 2.3.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1187
* Bump github.com/google/go-cmp from 0.5.9 to 0.6.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1189
* Bump golang.org/x/net from 0.16.0 to 0.17.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1190
* Bump go-version-input from 1.20.8 to 1.20.10 in govulncheck by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1195
* Bump golang from 1.21.2-bookworm to 1.21.3-bookworm in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1193
* Bump golang from 1.21.2-bookworm to 1.21.3-bookworm in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1191
* Bump golang from 1.21.2-bookworm to 1.21.3-bookworm in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1194
* Bump golang from 1.21.2-bookworm to 1.21.3-bookworm in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1192
* Bump golang from `a94b089` to `8f9a1ec` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1196
* Bump github/codeql-action from 2.22.1 to 2.22.2 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1197
* Bump golang from `a94b089` to `5cc7ddc` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1200
* Bump golang from `a94b089` to `5cc7ddc` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1199
* Bump github/codeql-action from 2.22.2 to 2.22.3 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1202
* Bump golang from `5cc7ddc` to `20f9ab5` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1203
* Bump golang from `a94b089` to `20f9ab5` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1198
* Bump golang from `8f9a1ec` to `20f9ab5` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1201
* Bump actions/checkout from 4.1.0 to 4.1.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1204
* Bump github/codeql-action from 2.22.3 to 2.22.4 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1206
* Bump ossf/scorecard-action from 2.3.0 to 2.3.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1207
* Bump github/codeql-action from 2.22.4 to 2.22.5 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1209
* Bump multiple Go module dependencies by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1213

## v1.1.6

### Dependency update

* Bump Trillian to v1.5.2
* Bump Prometheus to v0.43.1

## v1.1.5

### Public/Private Key Consistency

 * #1044: If a public key has been configured for a log, check that it is consistent with the private key.
 * #1046: Ensure that no two logs in the CTFE configuration use the same private key.

### Cleanup

 * Remove v2 log list package files.

### Misc

 * Updated golangci-lint to v1.51.1 (developers should update to this version).
 * Bump Go version from 1.17 to 1.19.

## v1.1.4

[Published 2022-10-21](https://github.com/google/certificate-transparency-go/releases/tag/v1.1.4)

### Cleanup

 * Remove log list v1 package and its dependencies.

### Migrillian

 * #960: Skip consistency check when root is size zero.

### Misc

 * Update Trillian to [0a389c4](https://github.com/google/trillian/commit/0a389c4bb8d97fb3be8f55d7e5b428cf4304986f)
 * Migrate loglist dependency from v1 to v3 in ctclient cmd.
 * Migrate loglist dependency from v1 to v3 in ctutil/loginfo.go
 * Migrate loglist dependency from v1 to v3 in ctutil/sctscan.go
 * Migrate loglist dependency from v1 to v3 in trillian/integration/ct_hammer/main.go
 * Downgrade 429 errors to verbosity 2

## v1.1.3

[Published 2022-05-14](https://github.com/google/certificate-transparency-go/releases/tag/v1.1.3)

### Integration

 * Breaking change to API for `integration.HammerCTLog`:
    * Added `ctx` as first argument, and terminate loop if it becomes cancelled

### JSONClient

 * PostAndParseWithRetry now does backoff-and-retry upon receiving HTTP 429.

### Cleanup

 * `WithBalancerName` is deprecated and removed, using the recommended way.
 * `ctfe.PEMCertPool` type has been moved to `x509util.PEMCertPool` to reduce
   dependencies (#903).

### Misc

 * updated golangci-lint to v1.46.1 (developers should update to this version)
 * update `google.golang.org/grpc` to v1.46.0
 * `ctclient` tool now uses Cobra for better CLI experience (#901).
 * #800: Remove dependency from `ratelimit`.
 * #927: Add read-only mode to CTFE config.

## v1.1.2

[Published 2021-09-21](https://github.com/google/certificate-transparency-go/releases/tag/v1.1.2)

### CTFE

 * Removed the `-by_range` flag.

### Updated dependencies

 * Trillian from v1.3.11 to v1.4.0
 * protobuf to v2

## v1.1.1

[Published 2020-10-06](https://github.com/google/certificate-transparency-go/releases/tag/v1.1.1)

### Tools

#### CT Hammer

Added a flag (--strict_sth_consistency_size) which when set to true enforces the current behaviour of only request consistency proofs between tree sizes for which the hammer has seen valid STHs.
When setting this flag to false, if no two usable STHs are available the hammer will attempt to request a consistency proof between the latest STH it's seen and a random smaller (but > 0) tree size.


### CTFE

#### Caching

The CTFE now includes a Cache-Control header in responses containing purely
immutable data, e.g. those for get-entries and get-proof-by-hash. This allows
clients and proxies to cache these responses for up to 24 hours.

#### EKU Filtering

> :warning: **It is not yet recommended to enable this option in a production CT Log!**

CTFE now supports filtering logging submissions by leaf certificate EKU.
This is enabled by adding an extKeyUsage list to a log's stanza in the
config file.

The format is a list of strings corresponding to the supported golang x509 EKUs:
  |Config string               | Extended Key Usage                     |
  |----------------------------|----------------------------------------|
  |`Any`                       |  ExtKeyUsageAny                        |
  |`ServerAuth`                |  ExtKeyUsageServerAuth                 |
  |`ClientAuth`                |  ExtKeyUsageClientAuth                 |
  |`CodeSigning`               |  ExtKeyUsageCodeSigning                |
  |`EmailProtection`           |  ExtKeyUsageEmailProtection            |
  |`IPSECEndSystem`            |  ExtKeyUsageIPSECEndSystem             |
  |`IPSECTunnel`               |  ExtKeyUsageIPSECTunnel                |
  |`IPSECUser`                 |  ExtKeyUsageIPSECUser                  |
  |`TimeStamping`              |  ExtKeyUsageTimeStamping               |
  |`OCSPSigning`               |  ExtKeyUsageOCSPSigning                |
  |`MicrosoftServerGatedCrypto`|  ExtKeyUsageMicrosoftServerGatedCrypto |
  |`NetscapeServerGatedCrypto` |  ExtKeyUsageNetscapeServerGatedCrypto  |

When an extKeyUsage list is specified, the CT Log will reject logging
submissions for leaf certificates that do not contain an EKU present in this
list.

When enabled, EKU filtering is only performed at the leaf level (i.e. there is
no 'nested' EKU filtering performed).

If no list is specified, or the list contains an `Any` entry, no EKU
filtering will be performed.

#### GetEntries
Calls to `get-entries` which are at (or above) the maximum permitted number of
entries whose `start` parameter does not fall on a multiple of the maximum
permitted number of entries, will have their responses truncated such that
subsequent requests will align with this boundary.
This is intended to coerce callers of `get-entries` into all using the same
`start` and `end` parameters and thereby increase the cacheability of
these requests.

e.g.:

<pre>
Old behaviour:
             1         2         3
             0         0         0
Entries>-----|---------|---------|----...
Client A -------|---------|----------|...
Client B --|--------|---------|-------...
           ^        ^         ^
           `--------`---------`---- requests

With coercion (max batch = 10 entries):
             1         2         3
             0         0         0
Entries>-----|---------|---------|----...
Client A ----X---------|---------|...
Client B --|-X---------|---------|-------...
             ^
             `-- Requests truncated
</pre>

This behaviour can be disabled by setting the `--align_getentries`
flag to false.

#### Flags

The `ct_server` binary changed the default of these flags:

-   `by_range` - Now defaults to `true`

The `ct_server` binary added the following flags:
-   `align_getentries` - See GetEntries section above for details

Added `backend` flag to `migrillian`, which now replaces the deprecated
"backend" feature of Migrillian configs.

#### FixedBackendResolver Replaced

This was previously used in situations where a comma separated list of
backends was provided in the `rpcBackend` flag rather than a single value.

It has been replaced by equivalent functionality using a newer gRPC API.
However this support was only intended for use in integration tests. In
production we recommend the use of etcd or a gRPC load balancer.

### LogList

Log list tools updated to use the correct v2 URL (from v2_beta previously).

### Libraries

#### x509 fork

Merged upstream Go 1.13 and Go 1.14 changes (with the exception
of https://github.com/golang/go/commit/14521198679e, to allow
old certs using a malformed root still to be logged).

#### asn1 fork

Merged upstream Go 1.14 changes.

#### ctutil

Added VerifySCTWithVerifier() to verify SCTs using a given ct.SignatureVerifier.

### Configuration Files

Configuration files that previously had to be text-encoded Protobuf messages can
now alternatively be binary-encoded instead.

### JSONClient

- `PostAndParseWithRetry` error logging now includes log URI in messages.

### Minimal Gossip Example

All the code for this, except for the x509ext package, has been moved over
to the [trillian-examples](https://github.com/google/trillian-examples) repository.

This keeps the code together and removes a circular dependency between the
two repositories. The package layout and structure remains the same so
updating should just mean changing any relevant import paths.

### Dependencies

A circular dependency on the [monologue](https://github.com/google/monologue) repository has been removed.

A circular dependency on the [trillian-examples](https://github.com/google/trillian-examples) repository has been removed.

The version of trillian in use has been updated to 1.3.11. This has required
various other dependency updates including gRPC and protobuf. This code now
uses the v2 proto API. The Travis tests now expect the 3.11.4 version of
protoc.

The version of etcd in use has been switched to the one from `go.etcd.io`.

Most of the above changes are to align versions more closely with the ones
used in the trillian repository.

## v1.1.0

Published 2019-11-14 15:00:00 +0000 UTC

### CTFE

The `reject_expired` and `reject_unexpired` configuration fields for the CTFE
have been changed so that their behaviour reflects their name:

-   `reject_expired` only rejects expired certificates (i.e. it now allows
    not-yet-valid certificates).
-   `reject_unexpired` only allows expired certificates (i.e. it now rejects
    not-yet-valid certificates).

A `reject_extensions` configuration field for the CTFE was added, this allows
submissions to be rejected if they contain an extension with any of the
specified OIDs.

A `frozen_sth` configuration field for the CTFE was added. This STH will be
served permanently. It must be signed by the log's private key.

A `/healthz` URL has been added which responds with HTTP 200 OK and the string
"ok" when the server is up.

#### Flags

The `ct_server` binary has these new flags:

-   `mask_internal_errors` - Removes error strings from HTTP 500 responses
    (Internal Server Error)

Removed default values for `--metrics_endpoint` and `--log_rpc_server` flags.
This makes it easier to get the documented "unset" behaviour.

#### Metrics

The CTFE exports these new metrics:

-   `is_mirror` - set to 1 for mirror logs (copies of logs hosted elsewhere)
-   `frozen_sth_timestamp` - time of the frozen Signed Tree Head in milliseconds
    since the epoch

#### Kubernetes

Updated prometheus-to-sd to v0.5.2.

A dedicated node pool is no longer required by the Kubernetes manifests.

### Log Lists

A new package has been created for parsing, searching and creating JSON log
lists compatible with the
[v2 schema](http://www.gstatic.com/ct/log_list/v2_beta/log_list_schema.json):
`github.com/google/certificate-transparency-go/loglist2`.

### Docker Images

Our Docker images have been updated to use Go 1.11 and
[Distroless base images](https://github.com/GoogleContainerTools/distroless).

The CTFE Docker image now sets `ENTRYPOINT`.

### Utilities / Libraries

#### jsonclient

The `jsonclient` package now copes with empty HTTP responses. The user-agent
header it sends can now be specified.

#### x509 and asn1 forks

Merged upstream changes from Go 1.12 into the `asn1` and `x509` packages.

Added a "lax" tag to `asn1` that applies recursively and makes some checks more
relaxed:

-   parsePrintableString() copes with invalid PrintableString contents, e.g. use
    of tagPrintableString when the string data is really ISO8859-1.
-   checkInteger() allows integers that are not minimally encoded (and so are
    not correct DER).
-   OIDs are allowed to be empty.

The following `x509` functions will now return `x509.NonFatalErrors` if ASN.1
parsing fails in strict mode but succeeds in lax mode. Previously, they only
attempted strict mode parsing.

-   `x509.ParseTBSCertificate()`
-   `x509.ParseCertificate()`
-   `x509.ParseCertificates()`

The `x509` package will now treat a negative RSA modulus as a non-fatal error.

The `x509` package now supports RSASES-OAEP and Ed25519 keys.

#### ctclient

The `ctclient` tool now defaults to using
[all_logs_list.json](https://www.gstatic.com/ct/log_list/all_logs_list.json)
instead of [log_list.json](https://www.gstatic.com/ct/log_list/log_list.json).
This can be overridden using the `--log_list` flag.

It can now perform inclusion checks on pre-certificates.

It has these new commands:

-   `bisect` - Finds a log entry given a timestamp.

It has these new flags:

-   `--chain` - Displays the entire certificate chain
-   `--dns_server` - The DNS server to direct queries to (system resolver by
    default)
-   `--skip_https_verify` - Skips verification of the HTTPS connection
-   `--timestamp` - Timestamp to use for `bisect` and `inclusion` commands (for
    `inclusion`, only if --leaf_hash is not used)

It now accepts hex or base64-encoded strings for the `--tree_hash`,
`--prev_hash` and `--leaf_hash` flags.

#### certcheck

The `certcheck` tool has these new flags:

-   `--check_time` - Check current validity of certificate (replaces
    `--timecheck`)
-   `--check_name` - Check validity of certificate name
-   `--check_eku` - Check validity of EKU nesting
-   `--check_path_len` - Check validity of path length constraint
-   `--check_name_constraint` - Check name constraints
-   `--check_unknown_critical_exts` - Check for unknown critical extensions
    (replaces `--ignore_unknown_critical_exts`)
-   `--strict` - Set non-zero exit code for non-fatal errors in parsing

#### sctcheck

The `sctcheck` tool has these new flags:

-   `--check_inclusion` - Checks that the SCT was honoured (i.e. the
    corresponding certificate was included in the issuing CT log)

#### ct_hammer

The `ct_hammer` tool has these new flags:

-   `--duplicate_chance` - Allows setting the probability of the hammer sending
    a duplicate submission.

## v1.0.21 - CTFE Logging / Path Options. Mirroring. RPKI. Non Fatal X.509 error improvements

Published 2018-08-20 10:11:04 +0000 UTC

### CTFE

`CTFE` no longer prints certificate chains as long byte strings in messages when handler errors occur. This was obscuring the reason for the failure and wasn't particularly useful.

`CTFE` now has a global log URL path prefix flag and a configuration proto for a log specific path. The latter should help for various migration strategies if existing C++ server logs are going to be converted to run on the new code.

### Mirroring

More progress has been made on log mirroring. We believe that it's now at the point where testing can begin.

### Utilities / Libraries

The `certcheck` and `ct_hammer` utilities have received more enhancements.

`x509` and `x509util` now support Subject Information Access and additional extensions for [RPKI / RFC 3779](https://www.ietf.org/rfc/rfc3779.txt).

`scanner` / `fixchain` and some other command line utilities now have better handling of non-fatal errors.

Commit [3629d6846518309d22c16fee15d1007262a459d2](https://api.github.com/repos/google/certificate-transparency-go/commits/3629d6846518309d22c16fee15d1007262a459d2) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.21)

## v1.0.20 - Minimal Gossip / Go 1.11 Fix / Utility Improvements

Published 2018-07-05 09:21:34 +0000 UTC

Enhancements have been made to various utilities including `scanner`, `sctcheck`, `loglist` and `x509util`.

The `allow_verification_with_non_compliant_keys` flag has been removed from `signatures.go`.

An implementation of Gossip has been added. See the `gossip/minimal` package for more information.

An X.509 compatibility issue for Go 1.11 has been fixed. This should be backwards compatible with 1.10.

Commit [37a384cd035e722ea46e55029093e26687138edf](https://api.github.com/repos/google/certificate-transparency-go/commits/37a384cd035e722ea46e55029093e26687138edf) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.20)

## v1.0.19 - CTFE User Quota

Published 2018-06-01 13:51:52 +0000 UTC

CTFE now supports Trillian Log's explicit quota API; quota can be requested based on the remote user's IP, as well as per-issuing certificate in submitted chains.

Commit [8736a411b4ff214ea20687e46c2b67d66ebd83fc](https://api.github.com/repos/google/certificate-transparency-go/commits/8736a411b4ff214ea20687e46c2b67d66ebd83fc) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.19)

## v1.0.18 - Adding Migration Tool / Client Additions / K8 Config

Published 2018-06-01 14:28:20 +0000 UTC

Work on a log migration tool (Migrillian) is in progress. This is not yet ready for production use but will provide features for mirroring and migrating logs.

The `RequestLog` API allows for logging of SCTs when they are issued by CTFE.

The CT Go client now supports `GetEntryAndProof`. Utilities have been switched over to use the `glog` package.

Commit [77abf2dac5410a62c04ac1c662c6d0fa54afc2dc](https://api.github.com/repos/google/certificate-transparency-go/commits/77abf2dac5410a62c04ac1c662c6d0fa54afc2dc) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.18)

## v1.0.17 - Merkle verification / Tracing / Demo script / CORS

Published 2018-06-01 14:25:16 +0000 UTC

Now uses Merkle Tree verification from Trillian.

The CT server now supports CORS.

Request tracing added using OpenCensus. For GCE / K8 it just requires the flag to be enabled to export traces to Stackdriver. Other environments may differ.

A demo script was added that goes through setting up a simple deployment suitable for development / demo purposes. This may be useful for those new to the project.

Commit [3c3d22ce946447d047a03228ebb4a41e3e4eb15b](https://api.github.com/repos/google/certificate-transparency-go/commits/3c3d22ce946447d047a03228ebb4a41e3e4eb15b) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.17)

## v1.0.16 - Lifecycle test / Go 1.10.1

Published 2018-06-01 14:22:23 +0000 UTC

An integration test was added that goes through a create / drain queue / freeze lifecycle for a log.

Changes to `x509` were merged from Go 1.10.1.

Commit [a72423d09b410b80673fd1135ba1022d04bac6cd](https://api.github.com/repos/google/certificate-transparency-go/commits/a72423d09b410b80673fd1135ba1022d04bac6cd) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.16)

## v1.0.15 - More control of verification, grpclb, stackdriver metrics

Published 2018-06-01 14:20:32 +0000 UTC

Facilities were added to the `x509` package to control whether verification checks are applied.

Log server requests are now balanced using `gRPClb`.

For Kubernetes, metrics can be published to Stackdriver monitoring.

Commit [684d6eee6092774e54d301ccad0ed61bc8d010c1](https://api.github.com/repos/google/certificate-transparency-go/commits/684d6eee6092774e54d301ccad0ed61bc8d010c1) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.15)

## v1.0.14 - SQLite Removed, LeafHashForLeaf

Published 2018-06-01 14:15:37 +0000 UTC

Support for SQLite was removed. This motivation was ongoing test flakiness caused by multi-user access. This database may work for an embedded scenario but is not suitable for use in a server environment.

A `LeafHashForLeaf` client API was added and is now used by the CT client and integration tests.

Commit [698cd6a661196db4b2e71437422178ffe8705006](https://api.github.com/repos/google/certificate-transparency-go/commits/698cd6a661196db4b2e71437422178ffe8705006) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.14)

## v1.0.13 - Crypto changes, util updates, sync with trillian repo, loglist verification

Published 2018-06-01 14:15:21 +0000 UTC

Some of our custom crypto package that were wrapping calls to the standard package have been removed and the base features used directly.

Updates were made to GCE ingress and health checks.

The log list utility can verify signatures.

Commit [480c3654a70c5383b9543ec784203030aedbd3a5](https://api.github.com/repos/google/certificate-transparency-go/commits/480c3654a70c5383b9543ec784203030aedbd3a5) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.13)

## v1.0.12 - Client / util updates & CTFE fixes

Published 2018-06-01 14:13:42 +0000 UTC

The CT client can now use a JSON loglist to find logs.

CTFE had a fix applied for preissued precerts.

A DNS client was added and CT client was extended to support DNS retrieval.

Commit [74c06c95e0b304a050a1c33764c8a01d653a16e3](https://api.github.com/repos/google/certificate-transparency-go/commits/74c06c95e0b304a050a1c33764c8a01d653a16e3) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.12)

## v1.0.11 - Kubernetes CI / Integration fixes

Published 2018-06-01 14:12:18 +0000 UTC

Updates to Kubernetes configs, mostly related to running a CI instance.

Commit [0856acca7e0ab7f082ae83a1fbb5d21160962efc](https://api.github.com/repos/google/certificate-transparency-go/commits/0856acca7e0ab7f082ae83a1fbb5d21160962efc) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.11)

## v1.0.10 - More scanner, x509, utility and client fixes. CTFE updates

Published 2018-06-01 14:09:47 +0000 UTC

The CT client was using the wrong protobuffer library package. To guard against this in future a check has been added to our lint config.

The `x509` and `asn1` packages have had upstream fixes applied from Go 1.10rc1.

Commit [1bec4527572c443752ad4f2830bef88be0533236](https://api.github.com/repos/google/certificate-transparency-go/commits/1bec4527572c443752ad4f2830bef88be0533236) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.10)

## v1.0.9 - Scanner, x509, utility and client fixes

Published 2018-06-01 14:11:13 +0000 UTC

The `scanner` utility now displays throughput stats.

Build instructions and README files were updated.

The `certcheck` utility can be told to ignore unknown critical X.509 extensions.

Commit [c06833528d04a94eed0c775104d1107bab9ae17c](https://api.github.com/repos/google/certificate-transparency-go/commits/c06833528d04a94eed0c775104d1107bab9ae17c) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.9)

## v1.0.8 - Client fixes, align with trillian repo

Published 2018-06-01 14:06:44 +0000 UTC



Commit [e8b02c60f294b503dbb67de0868143f5d4935e56](https://api.github.com/repos/google/certificate-transparency-go/commits/e8b02c60f294b503dbb67de0868143f5d4935e56) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.8)

## v1.0.7 - CTFE fixes

Published 2018-06-01 14:06:13 +0000 UTC

An issue was fixed with CTFE signature caching. In an unlikely set of circumstances this could lead to log mis-operation. While the chances of this are small, we recommend that versions prior to this one are not deployed.

Commit [52c0590bd3b4b80c5497005b0f47e10557425eeb](https://api.github.com/repos/google/certificate-transparency-go/commits/52c0590bd3b4b80c5497005b0f47e10557425eeb) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.7)

## v1.0.6 - crlcheck improvements / other fixes

Published 2018-06-01 14:04:22 +0000 UTC

The `crlcheck` utility has had several fixes and enhancements. Additionally the `hammer` now supports temporal logs.

Commit [3955e4a00c42e83ff17ce25003976159c5d0f0f9](https://api.github.com/repos/google/certificate-transparency-go/commits/3955e4a00c42e83ff17ce25003976159c5d0f0f9) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.6)

## v1.0.5 - X509 and asn1 fixes

Published 2018-06-01 14:02:58 +0000 UTC

This release is mostly fixes to the `x509` and `asn1` packages. Some command line utilities were also updated.

Commit [ae40d07cce12f1227c6e658e61c9dddb7646f97b](https://api.github.com/repos/google/certificate-transparency-go/commits/ae40d07cce12f1227c6e658e61c9dddb7646f97b) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.5)

## v1.0.4 - Multi log backend configs

Published 2018-06-01 14:02:07 +0000 UTC

Support was added to allow CTFE to use multiple backends, each serving a distinct set of logs. It allows for e.g. regional backend deployment with common frontend servers.

Commit [62023ed90b41fa40854957b5dec7d9d73594723f](https://api.github.com/repos/google/certificate-transparency-go/commits/62023ed90b41fa40854957b5dec7d9d73594723f) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.4)

## v1.0.3 - Hammer updates, use standard context

Published 2018-06-01 14:01:11 +0000 UTC

After the Go 1.9 migration references to anything other than the standard `context` package have been removed. This is the only one that should be used from now on.

Commit [b28beed8b9aceacc705e0ff4a11d435a310e3d97](https://api.github.com/repos/google/certificate-transparency-go/commits/b28beed8b9aceacc705e0ff4a11d435a310e3d97) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.3)

## v1.0.2 - Go 1.9

Published 2018-06-01 14:00:00 +0000 UTC

Go 1.9 is now required to build the code.

Commit [3aed33d672ee43f04b1e8a00b25ca3e2e2e74309](https://api.github.com/repos/google/certificate-transparency-go/commits/3aed33d672ee43f04b1e8a00b25ca3e2e2e74309) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.2)

## v1.0.1 - Hammer and client improvements

Published 2018-06-01 13:59:29 +0000 UTC



Commit [c28796cc21776667fb05d6300e32d9517be96515](https://api.github.com/repos/google/certificate-transparency-go/commits/c28796cc21776667fb05d6300e32d9517be96515) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0.1)

## v1.0 - First Trillian CT Release

Published 2018-06-01 13:59:00 +0000 UTC

This is the point that corresponds to the 1.0 release in the trillian repo.

Commit [abb79e468b6f3bbd48d1ab0c9e68febf80d52c4d](https://api.github.com/repos/google/certificate-transparency-go/commits/abb79e468b6f3bbd48d1ab0c9e68febf80d52c4d) Download [zip](https://api.github.com/repos/google/certificate-transparency-go/zipball/v1.0)
