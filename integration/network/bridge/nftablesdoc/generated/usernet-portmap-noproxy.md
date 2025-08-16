<!-- This is a generated file; DO NOT EDIT. -->

## Container on a user-defined network, with a published port, no userland proxy

Running the daemon with the userland proxy disabled then, as before, adding a
network running a container with a mapped port. Equivalent to:

    dockerd --userland-proxy=false
	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

Most rules are the same as the network with the [proxy enabled][0]:

<details>
<summary>Full table ...</summary>

    table ip docker-bridges {
    	map filter-forward-in-jumps {
    		type ifname : verdict
    		elements = { "docker0" : jump filter-forward-in__docker0,
    			     "bridge1" : jump filter-forward-in__bridge1 }
    	}
    
    	map filter-forward-out-jumps {
    		type ifname : verdict
    		elements = { "docker0" : jump filter-forward-out__docker0,
    			     "bridge1" : jump filter-forward-out__bridge1 }
    	}
    
    	map nat-postrouting-in-jumps {
    		type ifname : verdict
    		elements = { "docker0" : jump nat-postrouting-in__docker0,
    			     "bridge1" : jump nat-postrouting-in__bridge1 }
    	}
    
    	map nat-postrouting-out-jumps {
    		type ifname : verdict
    		elements = { "docker0" : jump nat-postrouting-out__docker0,
    			     "bridge1" : jump nat-postrouting-out__bridge1 }
    	}
    
    	chain filter-FORWARD {
    		type filter hook forward priority filter; policy accept;
    		oifname vmap @filter-forward-in-jumps
    		iifname vmap @filter-forward-out-jumps
    	}
    
    	chain nat-OUTPUT {
    		type nat hook output priority -100; policy accept;
    		fib daddr type local counter jump nat-prerouting-and-output
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
    		tcp dport 8080 counter dnat to 192.0.2.2:80 comment "DNAT"
    	}
    
    	chain raw-PREROUTING {
    		type filter hook prerouting priority raw; policy accept;
    		ip daddr 192.0.2.2 iifname != "bridge1" counter drop comment "DROP DIRECT ACCESS"
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
    		fib saddr type local counter masquerade comment "MASQUERADE FROM HOST"
    	}
    
    	chain nat-postrouting-out__docker0 {
    		oifname != "docker0" ip saddr 172.17.0.0/16 counter masquerade comment "MASQUERADE"
    	}
    
    	chain filter-forward-in__bridge1 {
    		ct state established,related counter accept
    		iifname "bridge1" counter accept comment "ICC"
    		ip daddr 192.0.2.2 tcp dport 80 counter accept
    		counter drop comment "UNPUBLISHED PORT DROP"
    	}
    
    	chain filter-forward-out__bridge1 {
    		ct state established,related counter accept
    		counter accept comment "OUTGOING"
    	}
    
    	chain nat-postrouting-in__bridge1 {
    		fib saddr type local counter masquerade comment "MASQUERADE FROM HOST"
    		ip saddr 192.0.2.2 ip daddr 192.0.2.2 tcp dport 80 counter masquerade comment "MASQ TO OWN PORT"
    	}
    
    	chain nat-postrouting-out__bridge1 {
    		oifname != "bridge1" ip saddr 192.0.2.0/24 counter masquerade comment "MASQUERADE"
    	}
    }
    

</details>

But ...

The jump from nat-OUTPUT chain to nat-prerouting-and-output happens for loopback
addresses, to DNAT packets from one network sent to a port published to the
loopback address by a container in another network - there's no proxy to catch
them ...

    	chain nat-OUTPUT {
    		type nat hook output priority -100; policy accept;
    		fib daddr type local counter jump nat-prerouting-and-output
    	}


The rule to DNAT from the host port to the container's port is not restricted
to packets from the network with the published port. Again, there's no proxy
to catch them:

    	chain nat-prerouting-and-output {
    		tcp dport 8080 counter dnat to 192.0.2.2:80 comment "DNAT"
    	}


The `nat-postrouting-in` chains have masquerade rules for packets sent from 
local addresses. And, the chain for bridge1 (which has a container with a published
port) has a masquerade rule for packets sent from the container to its own published
port on the host:

    	chain nat-postrouting-in__docker0 {
    		fib saddr type local counter masquerade comment "MASQUERADE FROM HOST"
    	}

    	chain nat-postrouting-in__bridge1 {
    		fib saddr type local counter masquerade comment "MASQUERADE FROM HOST"
    		ip saddr 192.0.2.2 ip daddr 192.0.2.2 tcp dport 80 counter masquerade comment "MASQ TO OWN PORT"
    	}


[0]: usernet-portmap.md
