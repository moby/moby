ipvlan networking surface area
==============================

Scope: the ipvlan network driver, as observable from outside the daemon. Every expectation here
holds on Linux nodes.

This spec is a stub. It holds expectations that belong to the ipvlan driver. They come from reviews
of the specs' coverage. It is not yet a complete account of the ipvlan driver.
[README.md](README.md) gives its conventions for tiers, vantage points and dependencies.
[network-common.md](network-common.md) covers what every network driver must do, including the
rows for a driver that does not accept an endpoint MAC address.

## 1. Endpoint options

| Applies to | Expectation | Tier | Observed from | Depends on |
| --- | --- | --- | --- | --- |
| An ipvlan network | `POST /containers/{id}/start` for a created or stopped container whose endpoint on that network sets `MacAddress` responds 500. `POST /networks/{id}/connect` to that network for a running container, with `MacAddress` set, at API version 1.54 or later, also responds 500. Both messages contain `ipvlan interfaces do not support custom mac address assignment`. See [§2](#2-known-divergences--non-goals). | C | `api` | `none` |

## 2. Known divergences & non-goals

| Topic | Current behavior | Depends on |
| --- | --- | --- |
| HTTP 500 for refused requests | The daemon refuses the requests below on purpose, but they respond 500. 500 is the status for a fault in the daemon. The first request is `POST /containers/{id}/start` for a container whose endpoint on an ipvlan network sets `MacAddress`. The second is `POST /networks/{id}/connect` to an ipvlan network for a running container, with `MacAddress` set, at API version 1.54 or later ([§1](#1-endpoint-options)). | `none` |
