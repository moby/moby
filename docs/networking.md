# Daemon networking configuration

The daemon configuration file (`daemon.json`) supports a `networking` section that groups network-related options. Each key mirrors a command-line flag or the legacy top-level fields used in earlier versions. Using the `networking` object keeps related settings together.

## Fields

The `networking` section accepts the following keys:

* `bridge` – name of the default bridge network interface (`--bridge`)
* `bip` / `bip6` – IPv4/IPv6 subnet for the default bridge (`--bip`, `--bip6`)
* `ip` – default host IP address for published ports (`--ip`)
* `fixed-cidr` / `fixed-cidr-v6` – restrict subnet space for containers (`--fixed-cidr`, `--fixed-cidr-v6`)
* `default-gateway` / `default-gateway-v6` – gateway addresses for the default bridge (`--default-gateway`, `--default-gateway-v6`)
* `icc` – enable inter-container communication on the default bridge (`--icc`)
* `ipv6` – enable IPv6 networking (`--ipv6`)
* `mtu` – MTU for the default bridge (`--mtu`)
* `ip-forward` / `ip-forward-no-drop` – control kernel IP forwarding and default FORWARD policy (`--ip-forward`, `--ip-forward-no-drop`)
* `ip-masq` – enable IP masquerading (`--ip-masq`)
* `iptables` / `ip6tables` – manage IPv4/IPv6 iptables rules (`--iptables`, `--ip6tables`)
* `userland-proxy` / `userland-proxy-path` – control the userland proxy (`--userland-proxy`, `--userland-proxy-path`)
* `allow-direct-routing` – enable direct routing when creating bridge networks (`--allow-direct-routing`)
* `bridge-accept-fwmark` – accept packets with the specified firewall mark (`--bridge-accept-fwmark`)
* `default-address-pools` – default address pools for new networks (`--default-address-pool`)
* `default-network-opts` – default driver options for new networks (`--default-network-opt`)
* `network-control-plane-mtu` – MTU for the overlay control plane (`--network-control-plane-mtu`)
* `firewall-backend` – firewall implementation to use (`--firewall-backend`)

## Legacy compatibility

All of the above keys may also be specified at the top level of `daemon.json` as in previous releases. When both the `networking` section and top-level fields are present, values in `networking` take precedence. The nested form is preferred for new installations and configuration management.

## Examples

Nested `networking` section:

```json
{
  "networking": {
    "bridge": "docker0",
    "bip": "172.17.0.1/16",
    "default-address-pools": [
      {"base": "172.30.0.0/16", "size": 24}
    ]
  }
}
```

Legacy top-level fields:

```json
{
  "bridge": "docker0",
  "bip": "172.17.0.1/16",
  "default-address-pools": [
    {"base": "172.30.0.0/16", "size": 24}
  ]
}
```
