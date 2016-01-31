# Changelog

## 0.6.0-rc6 (2016-01-30)
- Properly fixes https://github.com/docker/docker/issues/18814

## 0.6.0-rc5 (2016-01-26)
- Cleanup stale overlay sandboxes

## 0.6.0-rc4 (2016-01-25)
- Add Endpoints() API to Sandbox interface
- Fixed a race-condition in default gateway network creation

## 0.6.0-rc3 (2016-01-25)
- Fixes docker/docker#19576
- Fixed embedded DNS to listen in TCP as well
- Fixed a race-condition in IPAM to choose non-overlapping subnet for concurrent requests

## 0.6.0-rc2 (2016-01-21)
- Fixes docker/docker#19376
- Fixes docker/docker#15819
- Fixes libnetwork/#885, Not filter v6 DNS servers from resolv.conf
- Fixes docker/docker #19448, also handles the . in service and network names correctly.

## 0.6.0-rc1 (2016-01-14)
- Fixes docker/docker#19404
- Fixes the ungraceful daemon restart issue in systemd with remote network plugin
  (https://github.com/docker/libnetwork/issues/813)

## 0.5.6 (2016-01-14)
- Setup embedded DNS server correctly on container restart. Fixes docker/docker#19354

## 0.5.5 (2016-01-14)
- Allow network-scoped alias to be resolved for anonymous endpoint
- Self repair corrupted IP database that could happen in 1.9.0 & 1.9.1
- Skip IPTables cleanup if --iptables=false is set. Fixes docker/docker#19063

## 0.5.4 (2016-01-12)
- Removed the isNodeAlive protection when user forces an endpoint delete

## 0.5.3 (2016-01-12)
- Bridge driver supporting internal network option
- Backend implementation to support "force" option to network disconnect
- Fixing a regex in etchosts package to fix docker/docker#19080

## 0.5.2 (2016-01-08)
- Embedded DNS replacing /etc/hosts based Service Discovery
- Container local alias and Network-scoped alias support
- Backend support for internal network mode
- Support for IPAM driver options
- Fixes overlay veth cleanup issue : docker/docker#18814
- fixes docker/docker#19139
- disable IPv6 Duplicate Address Detection

## 0.5.1 (2015-12-07)
- Allowing user to assign IP Address for containers
- Fixes docker/docker#18214
- Fixes docker/docker#18380

## 0.5.0 (2015-10-30)

- Docker multi-host networking exiting experimental channel
- Introduced IP Address Management and IPAM drivers
- DEPRECATE service discovery from default bridge network
- Introduced new network UX
- Support for multiple networks in bridge driver
- Local persistance with boltdb

## 0.4.0 (2015-07-24)

- Introduce experimental version of Overlay driver
- Introduce experimental version of network plugins
- Introduce experimental version of network & service UX
- Introduced experimental /etc/hosts based service discovery
- Integrated with libkv
- Improving test coverage
- Fixed a bunch of issues with osl namespace mgmt

## 0.3.0 (2015-05-27)

- Introduce CNM (Container Networking Model)
- Replace docker networking with CNM & Bridge driver
