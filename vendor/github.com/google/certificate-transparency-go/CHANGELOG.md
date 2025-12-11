# CERTIFICATE-TRANSPARENCY-GO Changelog

## HEAD

## v1.3.2

### Misc

* [migrillian] remove etcd support in #1699
* Bump golangci-lint from 1.55.1 to 1.61.0 (developers should update to this version).
* Update ctclient tool to support SCT extensions field by @liweitianux in https://github.com/google/certificate-transparency-go/pull/1645
* Bump go to 1.23
* [ct_hammer] support HTTPS and Bearer token for Authentication.
* [preloader] support Bearer token Authentication for non temporal logs.
* [preloader] support end indexes
* [CTFE] Short cache max-age when get-entries returns fewer entries than requested by @robstradling in https://github.com/google/certificate-transparency-go/pull/1707
* [CTFE] Disalllow mismatching signature algorithm identifiers in #702.
* [jsonclient] surface HTTP Do and Read errors #1695 by @FiloSottile 

### CTFE Storage Saving: Extra Data Issuance Chain Deduplication

* Suppress unnecessary duplicate key errors in the IssuanceChainStorage PostgreSQL implementation by @robstradling in https://github.com/google/certificate-transparency-go/pull/1678
* Only store IssuanceChain if not cached by @robstradling in https://github.com/google/certificate-transparency-go/pull/1679

### CTFE Rate Limiting Of Non-Fresh Submissions

To protect a log from being flooded with requests for "old" certificates, optional rate limiting for "non-fresh submissions" can be configured by providing the following flags:

- `non_fresh_submission_age`
- `non_fresh_submission_burst`
- `non_fresh_submission_limit`

This can help to ensure that the log maintains its ability to (1) accept "fresh" submissions and (2) distribute all log entries to monitors.

* [CTFE] Configurable mechanism to rate-limit non-fresh submissions by @robstradling in https://github.com/google/certificate-transparency-go/pull/1698

### Dependency updates

*  Bump the docker-deps group across 5 directories with 3 updates (#1705)
*  Bump google.golang.org/grpc from 1.72.1 to 1.72.2 in the all-deps group (#1704)
*  Bump github.com/go-jose/go-jose/v4 in the go_modules group (#1700)
*  Bump the all-deps group with 7 updates (#1701)
*  Bump the all-deps group with 7 updates (#1693)
*  Bump the docker-deps group across 4 directories with 1 update (#1694)
*  Bump github/codeql-action from 3.28.13 to 3.28.16 in the all-deps group (#1692)
*  Bump the all-deps group across 1 directory with 7 updates (#1688)
*  Bump distroless/base-debian12 (#1686)
*  Bump golangci/golangci-lint-action from 6.5.1 to 7.0.0 in the all-deps group (#1685)
*  Bump the all-deps group with 4 updates (#1681)
*  Bump the all-deps group with 6 updates (#1683)
*  Bump the docker-deps group across 4 directories with 2 updates (#1682)
*  Bump github.com/golang-jwt/jwt/v4 in the go_modules group (#1680)
*  Bump golangci/golangci-lint-action in the all-deps group (#1676)
*  Bump the all-deps group with 2 updates (#1677)
*  Bump github/codeql-action from 3.28.10 to 3.28.11 in the all-deps group (#1670)
*  Bump the all-deps group with 8 updates (#1672)
*  Bump the docker-deps group across 4 directories with 1 update (#1671)
*  Bump the docker-deps group across 4 directories with 1 update (#1668)
*  Bump the all-deps group with 4 updates (#1666)
*  Bump golangci-lint from 1.55.1 to 1.61.0 (#1667)
*  Bump the all-deps group with 3 updates (#1665)
*  Bump github.com/spf13/cobra from 1.8.1 to 1.9.1 in the all-deps group (#1660)
*  Bump the docker-deps group across 5 directories with 2 updates (#1661)
*  Bump golangci/golangci-lint-action in the all-deps group (#1662)
*  Bump the docker-deps group across 4 directories with 1 update (#1656)
*  Bump the all-deps group with 2 updates (#1654)
*  Bump the all-deps group with 4 updates (#1657)
*  Bump github/codeql-action from 3.28.5 to 3.28.8 in the all-deps group (#1652)
*  Bump github.com/spf13/pflag from 1.0.5 to 1.0.6 in the all-deps group (#1651)
*  Bump the all-deps group with 2 updates (#1649)
*  Bump the all-deps group with 5 updates (#1650)
*  Bump the docker-deps group across 5 directories with 3 updates (#1648)
*  Bump google.golang.org/protobuf in the all-deps group (#1647)
*  Bump golangci/golangci-lint-action in the all-deps group (#1646)

## v1.3.1

* Add AllLogListSignatureURL by @AlexLaroche in https://github.com/google/certificate-transparency-go/pull/1634
* Add TiledLogs to log list JSON by @mcpherrinm in https://github.com/google/certificate-transparency-go/pull/1635
* chore: relax go directive to permit 1.22.x by @dnwe in https://github.com/google/certificate-transparency-go/pull/1640

### Dependency Update

* Bump github.com/fullstorydev/grpcurl from 1.9.1 to 1.9.2 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1627
* Bump the all-deps group with 3 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1628
* Bump the docker-deps group across 5 directories with 3 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1630
* Bump github/codeql-action from 3.27.5 to 3.27.6 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1629
* Bump golang.org/x/crypto from 0.30.0 to 0.31.0 in the go_modules group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1631
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1633
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1632
* Bump the docker-deps group across 4 directories with 1 update by @dependabot in https://github.com/google/certificate-transparency-go/pull/1638
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1637
* Bump the all-deps group across 1 directory with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1641
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1643
* Bump google.golang.org/grpc from 1.69.2 to 1.69.4 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1642

## v1.3.0

### CTFE Storage Saving: Extra Data Issuance Chain Deduplication

This feature now supports PostgreSQL, in addition to the support for MySQL/MariaDB that was added in [v1.2.0](#v1.2.0).

Log operators can choose to enable this feature for new PostgreSQL-based CT logs by adding new CTFE configs in the [LogMultiConfig](trillian/ctfe/configpb/config.proto) and importing the [database schema](trillian/ctfe/storage/postgresql/schema.sql). The other available options are documented in the [v1.2.0](#v1.2.0) changelog entry.

This change is tested in Cloud Build tests using the `postgres:17` Docker image as of the time of writing.

* Add IssuanceChainStorage PostgreSQL implementation by @robstradling in https://github.com/google/certificate-transparency-go/pull/1618

### Misc

* [Dependabot] Update all docker images in one PR by @mhutchinson in https://github.com/google/certificate-transparency-go/pull/1614
* Explicitly include version tag by @mhutchinson in https://github.com/google/certificate-transparency-go/pull/1617
* Add empty cloudbuild_postgresql.yaml by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1623

### Dependency update

* Bump the all-deps group with 4 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1609
* Bump golang from 1.23.2-bookworm to 1.23.3-bookworm in /internal/witness/cmd/feeder in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1611
* Bump github/codeql-action from 3.27.0 to 3.27.1 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1610
* Bump golang from 1.23.2-bookworm to 1.23.3-bookworm in /trillian/examples/deployment/docker/ctfe in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1612
* Bump github.com/golang-jwt/jwt/v4 from 4.5.0 to 4.5.1 in the go_modules group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1613
* Bump the docker-deps group across 3 directories with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1616
* Bump github/codeql-action from 3.27.1 to 3.27.2 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1615
* Bump the docker-deps group across 4 directories with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1622
* Bump github/codeql-action from 3.27.2 to 3.27.4 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1620
* Bump the all-deps group with 4 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1621
* Bump github.com/google/trillian from 1.6.1 to 1.7.0 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1624
* Bump github/codeql-action from 3.27.4 to 3.27.5 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1625

## v1.2.2

* Recommended Go version for development: 1.22
  * Using a different version can lead to presubmits failing due to unexpected diffs.

### Add TLS Support

Add TLS support for Trillian: By using `--trillian_tls_ca_cert_file` flag, users can provide a CA certificate, that is used to establish a secure communication with Trillian log server.

Add TLS support for ct_server: By using `--tls_certificate` and `--tls_key` flags, users can provide a service certificate and key, that enables the server to handle HTTPS requests.

* Add TLS support for CTLog server by @fghanmi in https://github.com/google/certificate-transparency-go/pull/1523
* Add TLS support for migrillian by @fghanmi in https://github.com/google/certificate-transparency-go/pull/1525
* fix TLS configuration for ct_server by @fghanmi in https://github.com/google/certificate-transparency-go/pull/1542
* Add Trillian TLS support for ct_server by @fghanmi in https://github.com/google/certificate-transparency-go/pull/1551

### HTTP Idle Connection Timeout Flag

A new flag `http_idle_timeout` is added to set the HTTP server's idle timeout value in the ct_server binary. This controls the maximum amount of time to wait for the next request when keep-alives are enabled.

* add flag for HTTP idle connection timeout value by @bobcallaway in https://github.com/google/certificate-transparency-go/pull/1597

### Misc

* Refactor issuance chain service by @mhutchinson in https://github.com/google/certificate-transparency-go/pull/1512
* Use the version in the go.mod file for vuln checks by @mhutchinson in https://github.com/google/certificate-transparency-go/pull/1528

### Fixes

* Fix failed tests on 32-bit OS by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1540

### Dependency update

* Bump go.etcd.io/etcd/v3 from 3.5.13 to 3.5.14 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1500
* Bump github/codeql-action from 3.25.6 to 3.25.7 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1501
* Bump golang.org/x/net from 0.25.0 to 0.26.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1503
* Group dependabot updates as much as possible by @mhutchinson in https://github.com/google/certificate-transparency-go/pull/1506
* Bump golang from 1.22.3-bookworm to 1.22.4-bookworm in /internal/witness/cmd/witness in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1507
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1511
* Bump golang from 1.22.3-bookworm to 1.22.4-bookworm in /trillian/examples/deployment/docker/ctfe in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1510
* Bump golang from 1.22.3-bookworm to 1.22.4-bookworm in /integration in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1509
* Bump golang from 1.22.3-bookworm to 1.22.4-bookworm in /internal/witness/cmd/feeder in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1508
* Bump the all-deps group with 3 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1516
* Bump golang from `aec4784` to `9678844` in /internal/witness/cmd/witness in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1518
* Bump alpine from 3.19 to 3.20 in /trillian/examples/deployment/docker/envsubst by @dependabot in https://github.com/google/certificate-transparency-go/pull/1492
* Bump golang from `aec4784` to `9678844` in /internal/witness/cmd/feeder in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1517
* Bump golang from `aec4784` to `9678844` in /trillian/examples/deployment/docker/ctfe in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1513
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1515
* Bump golang from `aec4784` to `9678844` in /integration in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1514
* Bump alpine from `77726ef` to `b89d9c9` in /trillian/examples/deployment/docker/envsubst in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1519
* Bump k8s.io/klog/v2 from 2.130.0 to 2.130.1 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1521
* Bump alpine from `77726ef` to `b89d9c9` in /internal/witness/cmd/feeder in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1520
* Bump github/codeql-action from 3.25.10 to 3.25.11 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1526
* Bump version of go used by the vuln checker by @mhutchinson in https://github.com/google/certificate-transparency-go/pull/1527
* Bump the all-deps group with 3 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1530
* Bump golang from 1.22.4-bookworm to 1.22.5-bookworm in /internal/witness/cmd/feeder in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1531
* Bump golang from 1.22.4-bookworm to 1.22.5-bookworm in /internal/witness/cmd/witness in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1532
* Bump the all-deps group in /trillian/examples/deployment/docker/ctfe with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1533
* Bump actions/upload-artifact from 4.3.3 to 4.3.4 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1534
* Bump golang from 1.22.4-bookworm to 1.22.5-bookworm in /integration in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1535
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1536
* Bump github/codeql-action from 3.25.12 to 3.25.13 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1538
* Bump the all-deps group with 3 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1537
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1543
* Bump golang from `6c27802` to `af9b40f` in /trillian/examples/deployment/docker/ctfe in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1544
* Bump golang from `6c27802` to `af9b40f` in /internal/witness/cmd/witness in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1548
* Bump golang from `6c27802` to `af9b40f` in /integration in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1547
* Bump alpine from `b89d9c9` to `0a4eaa0` in /trillian/examples/deployment/docker/envsubst in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1546
* Bump the all-deps group in /internal/witness/cmd/feeder with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1545
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1549
* Bump golang.org/x/time from 0.5.0 to 0.6.0 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1550
* Bump golang from 1.22.5-bookworm to 1.22.6-bookworm in /internal/witness/cmd/feeder in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1552
* Bump golang from 1.22.5-bookworm to 1.22.6-bookworm in /trillian/examples/deployment/docker/ctfe in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1553
* Bump golang from 1.22.5-bookworm to 1.22.6-bookworm in /integration in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1554
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1555
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1556
* Bump golang from 1.22.5-bookworm to 1.22.6-bookworm in /internal/witness/cmd/witness in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1557
* Bump github.com/prometheus/client_golang from 1.19.1 to 1.20.0 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1559
* Bump github/codeql-action from 3.26.0 to 3.26.3 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1561
* Bump golang from 1.22.6-bookworm to 1.23.0-bookworm in /internal/witness/cmd/witness in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1558
* Bump golang from 1.22.6-bookworm to 1.23.0-bookworm in /internal/witness/cmd/feeder in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1563
* Bump golang from 1.22.6-bookworm to 1.23.0-bookworm in /trillian/examples/deployment/docker/ctfe in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1560
* Bump golang from 1.22.6-bookworm to 1.23.0-bookworm in /integration in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1562
* Bump go version to 1.22.6 by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1564
* Bump github.com/prometheus/client_golang from 1.20.0 to 1.20.2 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1565
* Bump github/codeql-action from 3.26.3 to 3.26.5 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1566
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1568
* Bump the all-deps group with 3 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1569
* Bump go from 1.22.6 to 1.22.7 by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1574
* Bump alpine from `0a4eaa0` to `beefdbd` in /trillian/examples/deployment/docker/envsubst in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1571
* Bump the all-deps group across 1 directory with 5 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1577
* Bump golang from 1.23.0-bookworm to 1.23.1-bookworm in /internal/witness/cmd/witness in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1575
* Bump golang from 1.23.0-bookworm to 1.23.1-bookworm in /integration in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1576
* Bump the all-deps group in /trillian/examples/deployment/docker/ctfe with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1572
* Bump the all-deps group in /internal/witness/cmd/feeder with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1573
* Bump the all-deps group with 4 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1578
* Bump github/codeql-action from 3.26.6 to 3.26.7 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1579
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1580
* Bump github/codeql-action from 3.26.7 to 3.26.8 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1581
* Bump distroless/base-debian12 from `c925d12` to `88e0a2a` in /trillian/examples/deployment/docker/ctfe in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1582
* Bump the all-deps group in /trillian/examples/deployment/docker/ctfe with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1585
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1583
* Bump golang from `1a5326b` to `dba79eb` in /integration in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1584
* Bump golang from `1a5326b` to `dba79eb` in /internal/witness/cmd/feeder in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1587
* Bump golang from `1a5326b` to `dba79eb` in /internal/witness/cmd/witness in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1586
* Bump the all-deps group with 5 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1588
* Bump the all-deps group with 6 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1589
* Bump golang from 1.23.1-bookworm to 1.23.2-bookworm in /trillian/examples/deployment/docker/ctfe in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1593
* Bump golang from 1.23.1-bookworm to 1.23.2-bookworm in /integration in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1592
* Bump golang from 1.23.1-bookworm to 1.23.2-bookworm in /internal/witness/cmd/witness in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1591
* Bump golang from 1.23.1-bookworm to 1.23.2-bookworm in /internal/witness/cmd/feeder in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1590
* Bump the all-deps group with 2 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1595
* Bump github.com/prometheus/client_golang from 1.20.4 to 1.20.5 in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1598
* Bump golang from `18d2f94` to `2341ddf` in /integration in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1602
* Bump golang from `18d2f94` to `2341ddf` in /internal/witness/cmd/witness in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1599
* Bump golang from `18d2f94` to `2341ddf` in /trillian/examples/deployment/docker/ctfe in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1600
* Bump golang from `18d2f94` to `2341ddf` in /internal/witness/cmd/feeder in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1601
* Bump the all-deps group with 3 updates by @dependabot in https://github.com/google/certificate-transparency-go/pull/1603
* Bump distroless/base-debian12 from `6ae5fe6` to `8fe31fb` in /trillian/examples/deployment/docker/ctfe in the all-deps group by @dependabot in https://github.com/google/certificate-transparency-go/pull/1604

## v1.2.1

### Fixes

* Fix Go potential bugs and maintainability by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1496

### Dependency update

* Bump google.golang.org/grpc from 1.63.2 to 1.64.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1482

## v1.2.0

### CTFE Storage Saving: Extra Data Issuance Chain Deduplication

To reduce CT/Trillian database storage by deduplication of the entire issuance chain (intermediate certificate(s) and root certificate) that is currently stored in the Trillian merkle tree leaf ExtraData field. Storage cost should be reduced by at least 33% for new CT logs with this feature enabled. Currently only MySQL/MariaDB is supported to store the issuance chain in the CTFE database.

Existing logs are not affected by this change. 

Log operators can choose to opt-in this change for new CT logs by adding new CTFE configs in the [LogMultiConfig](trillian/ctfe/configpb/config.proto) and importing the [database schema](trillian/ctfe/storage/mysql/schema.sql). See [example](trillian/examples/deployment/docker/ctfe/ct_server.cfg).

- `ctfe_storage_connection_string`
- `extra_data_issuance_chain_storage_backend`

An optional LRU cache can be enabled by providing the following flags.

- `cache_type`
- `cache_size`
- `cache_ttl`

This change is tested in Cloud Build tests using the `mysql:8.4` Docker image as of the time of writing.

* Add issuance chain storage interface by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1430
* Add issuance chain cache interface by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1431
* Add CTFE extra data storage saving configs to config.proto by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1432
* Add new types `PrecertChainEntryHash` and `CertificateChainHash` for TLS marshal/unmarshal in storage saving by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1435
* Add IssuanceChainCache LRU implementation by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1454
* Add issuance chain service by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1452
* Add CTFE extra data storage saving configs validation by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1456
* Add IssuanceChainStorage MySQL implementation by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1462
* Fix errcheck lint in mysql test by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1464
* CTFE Extra Data Issuance Chain Deduplication by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1477
* Fix incorrect deployment doc and server config by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1494

### Submission proxy: Root compatibility checking

* Adds the ability for a CT client to disable root compatibile checking by @aaomidi in https://github.com/google/certificate-transparency-go/pull/1258

### Fixes

* Return 429 Too Many Requests for gRPC error code `ResourceExhausted` from Trillian by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1401
* Safeguard against redirects on PUT request by @mhutchinson in https://github.com/google/certificate-transparency-go/pull/1418
* Fix CT client upload to be safe against no-op POSTs by @mhutchinson in https://github.com/google/certificate-transparency-go/pull/1424

### Misc

* Prefix errors.New variables with the word "Err" by @aaomidi in https://github.com/google/certificate-transparency-go/pull/1399
* Remove lint exceptions and fix remaining issues by @silaselisha in https://github.com/google/certificate-transparency-go/pull/1438
* Fix invalid Go toolchain version by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1471
* Regenerate proto files by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1489

### Dependency update

* Bump distroless/base-debian12 from `5eae9ef` to `28a7f1f` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1388
* Bump github/codeql-action from 3.24.6 to 3.24.7 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1389
* Bump actions/checkout from 4.1.1 to 4.1.2 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1390
* Bump golang from `6699d28` to `7f9c058` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1391
* Bump golang from `6699d28` to `7f9c058` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1392
* Bump golang from `6699d28` to `7a392a2` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1393
* Bump golang from `6699d28` to `7a392a2` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1394
* Bump golang from `7a392a2` to `d996c64` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1395
* Bump golang from `7f9c058` to `d996c64` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1396
* Bump golang from `7a392a2` to `d996c64` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1397
* Bump golang from `7f9c058` to `d996c64` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1398
* Bump github/codeql-action from 3.24.7 to 3.24.8 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1400
* Bump github/codeql-action from 3.24.8 to 3.24.9 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1402
* Bump go.etcd.io/etcd/v3 from 3.5.12 to 3.5.13 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1405
* Bump distroless/base-debian12 from `28a7f1f` to `611d30d` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1406
* Bump golang from 1.22.1-bookworm to 1.22.2-bookworm in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1407
* Bump golang.org/x/net from 0.22.0 to 0.23.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1408
* update govulncheck go version from 1.21.8 to 1.21.9 by @phbnf in https://github.com/google/certificate-transparency-go/pull/1412
* Bump golang from 1.22.1-bookworm to 1.22.2-bookworm in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1409
* Bump golang from 1.22.1-bookworm to 1.22.2-bookworm in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1410
* Bump golang.org/x/crypto from 0.21.0 to 0.22.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1414
* Bump golang from 1.22.1-bookworm to 1.22.2-bookworm in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1411
* Bump github/codeql-action from 3.24.9 to 3.24.10 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1415
* Bump golang.org/x/net from 0.23.0 to 0.24.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1416
* Bump google.golang.org/grpc from 1.62.1 to 1.63.2 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1417
* Bump github.com/fullstorydev/grpcurl from 1.8.9 to 1.9.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1419
* Bump golang from `48b942a` to `3451eec` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1421
* Bump golang from `48b942a` to `3451eec` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1423
* Bump golang from `48b942a` to `3451eec` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1420
* Bump golang from `3451eec` to `b03f3ba` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1426
* Bump golang from `3451eec` to `b03f3ba` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1425
* Bump golang from `48b942a` to `3451eec` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1422
* Bump golang from `3451eec` to `b03f3ba` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1427
* Bump golang from `3451eec` to `b03f3ba` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1428
* Bump github/codeql-action from 3.24.10 to 3.25.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1433
* Bump github/codeql-action from 3.25.0 to 3.25.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1434
* Bump actions/upload-artifact from 4.3.1 to 4.3.2 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1436
* Bump actions/checkout from 4.1.2 to 4.1.3 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1437
* Bump actions/upload-artifact from 4.3.2 to 4.3.3 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1440
* Bump github/codeql-action from 3.25.1 to 3.25.2 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1441
* Bump golang from `b03f3ba` to `d0902ba` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1444
* Bump golang from `b03f3ba` to `d0902ba` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1443
* Bump github.com/rs/cors from 1.10.1 to 1.11.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1442
* Bump golang from `b03f3ba` to `d0902ba` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1447
* Bump actions/checkout from 4.1.3 to 4.1.4 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1446
* Bump github/codeql-action from 3.25.2 to 3.25.3 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1449
* Bump golangci/golangci-lint-action from 4.0.0 to 5.0.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1448
* Bump golang from `b03f3ba` to `d0902ba` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1445
* Bump golangci/golangci-lint-action from 5.0.0 to 5.1.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1451
* Bump distroless/base-debian12 from `611d30d` to `d8d01e2` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1450
* Bump google.golang.org/protobuf from 1.33.1-0.20240408130810-98873a205002 to 1.34.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1453
* Bump actions/setup-go from 5.0.0 to 5.0.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1455
* Bump golang.org/x/net from 0.24.0 to 0.25.0 and golang.org/x/crypto from v0.22.0 to v0.23.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1457
* Bump google.golang.org/protobuf from 1.34.0 to 1.34.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1458
* Bump distroless/base-debian12 from `d8d01e2` to `786007f` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1461
* Bump golangci/golangci-lint-action from 5.1.0 to 5.3.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1460
* Bump `go-version-input` to 1.21.10 in govulncheck.yml by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1472
* Bump golangci/golangci-lint-action from 5.3.0 to 6.0.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1473
* Bump actions/checkout from 4.1.4 to 4.1.5 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1469
* Bump github.com/go-sql-driver/mysql from 1.7.1 to 1.8.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1465
* Bump golang from 1.22.2-bookworm to 1.22.3-bookworm in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1466
* Bump golang from 1.22.2-bookworm to 1.22.3-bookworm in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1463
* Bump golang from 1.22.2-bookworm to 1.22.3-bookworm in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1470
* Bump golang from 1.22.2-bookworm to 1.22.3-bookworm in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1467
* Bump github/codeql-action from 3.25.3 to 3.25.4 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1474
* Bump github.com/prometheus/client_golang from 1.19.0 to 1.19.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1475
* Bump ossf/scorecard-action from 2.3.1 to 2.3.3 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1476
* Bump github/codeql-action from 3.25.4 to 3.25.5 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1478
* Bump golang from `6d71b7c` to `ef27a3c` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1480
* Bump golang from `6d71b7c` to `ef27a3c` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1481
* Bump golang from `6d71b7c` to `ef27a3c` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1479
* Bump golang from `6d71b7c` to `ef27a3c` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1483
* Bump golang from `ef27a3c` to `5c56bd4` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1484
* Bump golang from `ef27a3c` to `5c56bd4` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1485
* Bump golang from `ef27a3c` to `5c56bd4` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1486
* Bump actions/checkout from 4.1.5 to 4.1.6 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1487
* Bump golang from `ef27a3c` to `5c56bd4` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1488
* Bump github/codeql-action from 3.25.5 to 3.25.6 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1490
* Bump alpine from `c5b1261` to `58d02b4` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1491
* Bump alpine from `58d02b4` to `77726ef` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1493

## v1.1.8

* Recommended Go version for development: 1.21
  * Using a different version can lead to presubmits failing due to unexpected diffs.

### Add support for AIX

* crypto/x509: add AIX operating system by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1277

### Monitoring

* Distribution metric to monitor the start of get-entries requests by @phbnf in https://github.com/google/certificate-transparency-go/pull/1364

### Fixes

* Use the appropriate HTTP response code for backend timeouts by @robstradling in https://github.com/google/certificate-transparency-go/pull/1313

### Misc

* Move golangci-lint from Cloud Build to GitHub Action by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1230
* Set golangci-lint GH action timeout to 5m by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1231
* Added Slack channel details by @mhutchinson in https://github.com/google/certificate-transparency-go/pull/1246
* Improve fuzzing by @AdamKorcz in https://github.com/google/certificate-transparency-go/pull/1345

### Dependency update

* Bump golang from `20f9ab5` to `5ee1296` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1216
* Bump golang from `20f9ab5` to `5ee1296` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1217
* Bump golang from `20f9ab5` to `5ee1296` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1218
* Bump k8s.io/klog/v2 from 2.100.1 to 2.110.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1219
* Bump golang from `20f9ab5` to `5ee1296` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1220
* Bump golang from `5ee1296` to `5bafbbb` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1221
* Bump golang from `5ee1296` to `5bafbbb` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1222
* Bump golang from `5ee1296` to `5bafbbb` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1223
* Bump golang from `5ee1296` to `5bafbbb` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1224
* Update the minimal image to gcr.io/distroless/base-debian12 by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1148
* Bump jq from 1.6 to 1.7 by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1225
* Bump github.com/spf13/cobra from 1.7.0 to 1.8.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1226
* Bump golang.org/x/time from 0.3.0 to 0.4.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1227
* Bump github.com/mattn/go-sqlite3 from 1.14.17 to 1.14.18 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1228
* Bump github.com/gorilla/mux from 1.8.0 to 1.8.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1229
* Bump golang from 1.21.3-bookworm to 1.21.4-bookworm in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1232
* Bump golang from 1.21.3-bookworm to 1.21.4-bookworm in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1233
* Bump golang from 1.21.3-bookworm to 1.21.4-bookworm in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1234
* Bump golang from 1.21.3-bookworm to 1.21.4-bookworm in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1235
* Bump go-version-input from 1.20.10 to 1.20.11 in govulncheck.yml by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1238
* Bump golang.org/x/net from 0.17.0 to 0.18.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1236
* Bump github/codeql-action from 2.22.5 to 2.22.6 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1240
* Bump github/codeql-action from 2.22.6 to 2.22.7 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1241
* Bump golang from `85aacbe` to `dadce81` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1243
* Bump golang from `85aacbe` to `dadce81` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1242
* Bump golang from `85aacbe` to `dadce81` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1244
* Bump golang from `85aacbe` to `dadce81` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1245
* Bump golang from `dadce81` to `52362e2` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1247
* Bump golang from `dadce81` to `52362e2` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1248
* Bump golang from `dadce81` to `52362e2` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1249
* Bump golang from `dadce81` to `52362e2` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1250
* Bump github/codeql-action from 2.22.7 to 2.22.8 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1251
* Bump golang.org/x/net from 0.18.0 to 0.19.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1252
* Bump golang.org/x/time from 0.4.0 to 0.5.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1254
* Bump alpine from `eece025` to `34871e7` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1256
* Bump alpine from `eece025` to `34871e7` in /trillian/examples/deployment/docker/envsubst by @dependabot in https://github.com/google/certificate-transparency-go/pull/1257
* Bump go-version-input from 1.20.11 to 1.20.12 in govulncheck.yml by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1264
* Bump actions/setup-go from 4.1.0 to 5.0.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1261
* Bump golang from 1.21.4-bookworm to 1.21.5-bookworm in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1259
* Bump golang from 1.21.4-bookworm to 1.21.5-bookworm in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1263
* Bump golang from 1.21.4-bookworm to 1.21.5-bookworm in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1262
* Bump golang from 1.21.4-bookworm to 1.21.5-bookworm in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1260
* Bump go.etcd.io/etcd/v3 from 3.5.10 to 3.5.11 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1266
* Bump github/codeql-action from 2.22.8 to 2.22.9 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1269
* Bump alpine from `34871e7` to `51b6726` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1270
* Bump alpine from 3.18 to 3.19 in /trillian/examples/deployment/docker/envsubst by @dependabot in https://github.com/google/certificate-transparency-go/pull/1271
* Bump golang from `a6b787c` to `2d3b13c` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1272
* Bump golang from `a6b787c` to `2d3b13c` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1273
* Bump golang from `a6b787c` to `2d3b13c` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1274
* Bump golang from `a6b787c` to `2d3b13c` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1275
* Bump github/codeql-action from 2.22.9 to 2.22.10 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1278
* Bump google.golang.org/grpc from 1.59.0 to 1.60.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1279
* Bump github/codeql-action from 2.22.10 to 3.22.11 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1280
* Bump distroless/base-debian12 from `1dfdb5e` to `8a0bb63` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1281
* Bump github.com/google/trillian from 1.5.3 to 1.5.4-0.20240110091238-00ca9abe023d by @mhutchinson in https://github.com/google/certificate-transparency-go/pull/1297
* Bump actions/upload-artifact from 3.1.3 to 4.0.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1282
* Bump github/codeql-action from 3.22.11 to 3.23.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1295
* Bump github.com/mattn/go-sqlite3 from 1.14.18 to 1.14.19 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1283
* Bump golang from 1.21.5-bookworm to 1.21.6-bookworm in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1300
* Bump distroless/base-debian12 from `8a0bb63` to `0a93daa` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1284
* Bump golang from 1.21.5-bookworm to 1.21.6-bookworm in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1299
* Bump golang from 1.21.5-bookworm to 1.21.6-bookworm in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1298
* Bump golang from 1.21.5-bookworm to 1.21.6-bookworm in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1301
* Bump golang from `688ad7f` to `1e8ea75` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1306
* Bump golang from `688ad7f` to `1e8ea75` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1305
* Use trillian release instead of pinned commit by @mhutchinson in https://github.com/google/certificate-transparency-go/pull/1304
* Bump actions/upload-artifact from 4.0.0 to 4.1.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1310
* Bump golang from `1e8ea75` to `cbee5d2` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1312
* Bump golang from `688ad7f` to `cbee5d2` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1308
* Bump golang from `1e8ea75` to `cbee5d2` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1311
* Bump golang.org/x/net from 0.19.0 to 0.20.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1302
* Bump golang from `b651ed8` to `cbee5d2` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1309
* Bump golang from `cbee5d2` to `c4b696f` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1314
* Bump golang from `cbee5d2` to `c4b696f` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1315
* Bump github/codeql-action from 3.23.0 to 3.23.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1317
* Bump golang from `cbee5d2` to `c4b696f` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1316
* Bump golang from `cbee5d2` to `c4b696f` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1318
* Bump k8s.io/klog/v2 from 2.120.0 to 2.120.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1319
* Bump actions/upload-artifact from 4.1.0 to 4.2.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1320
* Bump actions/upload-artifact from 4.2.0 to 4.3.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1321
* Bump golang from `c4b696f` to `d8c365d` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1326
* Bump golang from `c4b696f` to `d8c365d` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1323
* Bump google.golang.org/grpc from 1.60.1 to 1.61.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1324
* Bump golang from `c4b696f` to `d8c365d` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1322
* Bump golang from `c4b696f` to `d8c365d` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1325
* Bump github.com/mattn/go-sqlite3 from 1.14.19 to 1.14.20 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1327
* Bump github/codeql-action from 3.23.1 to 3.23.2 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1328
* Bump alpine from `51b6726` to `c5b1261` in /trillian/examples/deployment/docker/envsubst by @dependabot in https://github.com/google/certificate-transparency-go/pull/1330
* Bump alpine from `51b6726` to `c5b1261` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1329
* Bump go.etcd.io/etcd/v3 from 3.5.11 to 3.5.12 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1332
* Bump github.com/mattn/go-sqlite3 from 1.14.20 to 1.14.21 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1333
* Bump golang from `d8c365d` to `69bfed3` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1335
* Bump golang from `d8c365d` to `69bfed3` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1338
* Bump golang from `d8c365d` to `69bfed3` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1337
* Bump golang from `d8c365d` to `69bfed3` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1336
* Bump golang from `69bfed3` to `3efef61` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1339
* Bump github.com/mattn/go-sqlite3 from 1.14.21 to 1.14.22 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1344
* Bump golang from `69bfed3` to `3efef61` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1341
* Bump golang from `69bfed3` to `3efef61` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1343
* Bump distroless/base-debian12 from `0a93daa` to `f47fa3d` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1340
* Bump golang from `69bfed3` to `3efef61` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1342
* Bump github/codeql-action from 3.23.2 to 3.24.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1346
* Bump actions/upload-artifact from 4.3.0 to 4.3.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1347
* Bump golang from 1.21.6-bookworm to 1.22.0-bookworm in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1350
* Bump golang from 1.21.6-bookworm to 1.22.0-bookworm in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1348
* Bump golang from 1.21.6-bookworm to 1.22.0-bookworm in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1349
* Bump golang from 1.21.6-bookworm to 1.22.0-bookworm in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1351
* Bump golang.org/x/crypto from 0.18.0 to 0.19.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1353
* Bump golangci/golangci-lint-action from 3.7.0 to 4.0.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1354
* Bump golang.org/x/net from 0.20.0 to 0.21.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1352
* Bump distroless/base-debian12 from `f47fa3d` to `2102ce1` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1355
* Bump github/codeql-action from 3.24.0 to 3.24.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1357
* Bump golang from `874c267` to `5a3e169` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1356
* Bump golang from `874c267` to `5a3e169` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1358
* Bump golang from `874c267` to `5a3e169` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1359
* Bump golang from `874c267` to `5a3e169` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1360
* Bump github/codeql-action from 3.24.1 to 3.24.3 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1366
* Bump golang from `5a3e169` to `925fe3f` in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1363
* Bump google.golang.org/grpc from 1.61.0 to 1.61.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1362
* Bump golang from `5a3e169` to `925fe3f` in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1365
* Bump golang from `5a3e169` to `925fe3f` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1361
* Bump golang from `5a3e169` to `925fe3f` in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1367
* Bump golang/govulncheck-action from 1.0.1 to 1.0.2 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1368
* Bump github/codeql-action from 3.24.3 to 3.24.5 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1371
* Bump google.golang.org/grpc from 1.61.1 to 1.62.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1369
* Bump distroless/base-debian12 from `2102ce1` to `5eae9ef` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1372
* Bump distroless/base-debian12 from `5eae9ef` to `f9b0e86` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1375
* Bump golang.org/x/crypto from 0.19.0 to 0.20.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1374
* Bump github.com/prometheus/client_golang from 1.18.0 to 1.19.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1373
* Bump github/codeql-action from 3.24.5 to 3.24.6 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1377
* Bump distroless/base-debian12 from `f9b0e86` to `5eae9ef` in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1376
* Bump golang.org/x/net from 0.21.0 to 0.22.0 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1378
* Bump Go from 1.20 to 1.21 by @roger2hk in https://github.com/google/certificate-transparency-go/pull/1386
* Bump google.golang.org/grpc from 1.62.0 to 1.62.1 by @dependabot in https://github.com/google/certificate-transparency-go/pull/1380
* Bump golang from 1.22.0-bookworm to 1.22.1-bookworm in /integration by @dependabot in https://github.com/google/certificate-transparency-go/pull/1382
* Bump golang from 1.22.0-bookworm to 1.22.1-bookworm in /trillian/examples/deployment/docker/ctfe by @dependabot in https://github.com/google/certificate-transparency-go/pull/1385
* Bump golang from 1.22.0-bookworm to 1.22.1-bookworm in /internal/witness/cmd/witness by @dependabot in https://github.com/google/certificate-transparency-go/pull/1384
* Bump golang from 1.22.0-bookworm to 1.22.1-bookworm in /internal/witness/cmd/feeder by @dependabot in https://github.com/google/certificate-transparency-go/pull/1383

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
