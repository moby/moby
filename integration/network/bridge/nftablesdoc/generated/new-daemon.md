<!-- This is a generated file; DO NOT EDIT. -->

## nftables for a new Daemon

When the daemon starts, it creates two tables, `ip docker-bridges` and
`ip6 docker-bridges` for IPv4 and IPv6 rules respectively. Each table contains
some base chains and empty verdict maps. Rules for the default bridge network
are then added.

    table ip docker-bridges {
    	map filter-forward-in-jumps {
    		type ifname : verdict
    		elements = { "docker0" : jump filter-forward-in__docker0 }
    	}
    
    	map filter-forward-out-jumps {
    		type ifname : verdict
    		elements = { "docker0" : jump filter-forward-out__docker0 }
    	}
    
    	map nat-postrouting-in-jumps {
    		type ifname : verdict
    		elements = { "docker0" : jump nat-postrouting-in__docker0 }
    	}
    
    	map nat-postrouting-out-jumps {
    		type ifname : verdict
    		elements = { "docker0" : jump nat-postrouting-out__docker0 }
    	}
    
    	chain filter-FORWARD {
    		type filter hook forward priority filter; policy accept;
    		oifname vmap @filter-forward-in-jumps
    		iifname vmap @filter-forward-out-jumps
    	}
    
    	chain nat-OUTPUT {
    		type nat hook output priority -100; policy accept;
    		ip daddr != 127.0.0.0/8 fib daddr type local counter jump nat-prerouting-and-output
    	}
    
    	chain nat-POSTROUTING {
    		type nat hook postrouting priority srcnat; policy accept;
    		iifname vmap @nat-postrouting-out-jumps
    		oifname vmap @nat-postrouting-in-jumps
    	}
    
    	chain nat-PREROUTING {
    		type nat hook prerouting priority dstnat; policy accept;
    		fib daddr type local counter jump nat-prerouting-and-output
    	}
    
    	chain nat-prerouting-and-output {
    	}
    
    	chain raw-PREROUTING {
    		type filter hook prerouting priority raw; policy accept;
    	}
    
    	chain filter-forward-in__docker0 {
    		ct state established,related counter accept
    		iifname "docker0" counter accept comment "ICC"
    		counter drop comment "UNPUBLISHED PORT DROP"
    	}
    
    	chain filter-forward-out__docker0 {
    		ct state established,related counter accept
    		counter accept comment "OUTGOING"
    	}
    
    	chain nat-postrouting-in__docker0 {
    	}
    
    	chain nat-postrouting-out__docker0 {
    		oifname != "docker0" ip saddr 172.17.0.0/16 counter masquerade comment "MASQUERADE"
    	}
    }
    

#### filter-FORWARD

Chain `filter-FORWARD` is a base chain, with type `filter` and hook `forward`.
_So, it's equivalent to the iptables built-in chain `FORWARD` in the `filter`
table._ It's initialised with two rules that use the output and input
interface names as keys in verdict maps:

    	chain filter-FORWARD {
    		type filter hook forward priority filter; policy accept;
    		oifname vmap @filter-forward-in-jumps
    		iifname vmap @filter-forward-out-jumps
    	}


The verdict maps will be populated with an element per bridge network, each
jumping to a chain containing rules for that bridge. (So, for packets that
aren't going to-or-from a Docker bridge device, no jump rules are found in
the verdict map, and the packets don't need any further processing by this
base chain.)

The filter-FORWARD chain's policy shown above is `accept`. However:

   - For IPv4, the policy is `drop` if the sysctl
     net.ipv4.ip_forward was not set to '1', and the daemon set it itself when
     an IPv4-enabled bridge network was created.
   - For IPv6, similar, but for sysctls "/proc/sys/net/ipv6/conf/default/forwarding"
     and "/proc/sys/net/ipv6/conf/all/forwarding".

#### Per-network filter-FORWARD rules

Chains added for the default bridge network are named after the base chain
hook they're called from, and the network's bridge.

Packets processed by `filter-forward-in__*` will be delivered to the bridge
network if accepted. For docker0, the chain is:

    	chain filter-forward-in__docker0 {
    		ct state established,related counter accept
    		iifname "docker0" counter accept comment "ICC"
    		counter drop comment "UNPUBLISHED PORT DROP"
    	}


The rules are:
- conntrack accept for established flows. _Note that accept only applies to the
  base chain, accepted packets may be processed by other base chains registered
  with the same hook._
- accept packets originating within the network, because inter-container
  communication (ICC) is enabled.
- drop any other packets, because there are no containers in the network
  with published ports. _This means there is no dependency on the filter-FORWARD
  chain's default policy. Even if it is ACCEPT, packets will be dropped unless
  container ports/protocols are published._

Packets processed by `filter-forward-out__*` originate from the bridge network:

    	chain filter-forward-out__docker0 {
    		ct state established,related counter accept
    		counter accept comment "OUTGOING"
    	}


The rules in docker0's chain are:
- conntrack accept for established flows.
- an accept rule, containers in this network have access to external networks.

#### nat-POSTROUTING

Like the filter-FORWARD chain, nat-POSTROUTING has a jump to per-network chains
for packets to and from the network.

    	chain nat-POSTROUTING {
    		type nat hook postrouting priority srcnat; policy accept;
    		iifname vmap @nat-postrouting-out-jumps
    		oifname vmap @nat-postrouting-in-jumps
    	}


#### Per-network nat-POSTROUTING rules

In docker0's nat-postrouting chains, there's a single masquerade rule for packets
leaving the network:

    	chain nat-postrouting-in__docker0 {
    	}

    	chain nat-postrouting-out__docker0 {
    		oifname != "docker0" ip saddr 172.17.0.0/16 counter masquerade comment "MASQUERADE"
    	}

