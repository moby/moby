#!/usr/bin/env bash

set -eux -o pipefail

cat << EOF > /etc/modules-load.d/docker.conf
br_netfilter
bridge
ip6_tables
ip6table_filter
ip6table_nat
ip_tables
ip_vs
iptable_filter
iptable_nat
nf_tables
overlay
tap
tun
veth
x_tables
xt_addrtype
xt_comment
xt_conntrack
xt_mark
xt_multiport
xt_nat
xt_tcpudp
EOF

systemctl restart systemd-modules-load.service
