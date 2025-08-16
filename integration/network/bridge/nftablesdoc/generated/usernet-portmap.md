<!-- This is a generated file; DO NOT EDIT. -->

## Container on a user-defined network, with a published port

Adding a network running a container with a mapped port, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

The `ip docker-bridges` table is updated as follows:

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
    		iifname != "bridge1" tcp dport 8080 counter dnat to 192.0.2.2:80 comment "DNAT"
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
    	}
    
    	chain nat-postrouting-out__bridge1 {
    		oifname != "bridge1" ip saddr 192.0.2.0/24 counter masquerade comment "MASQUERADE"
    	}
    }
    

The new network has its own set of chains, similar to the chains for docker0 (which
doesn't have any containers), but ...

#### Published port

In chain `filter-forward-in__bridge1`, there's a rule to open the container's
published port:

    	chain filter-forward-in__bridge1 {
    		ct state established,related counter accept
    		iifname "bridge1" counter accept comment "ICC"
    		ip daddr 192.0.2.2 tcp dport 80 counter accept
    		counter drop comment "UNPUBLISHED PORT DROP"
    	}


A rule in `raw-PREROUTING` makes sure the container's published port cannot be
accessed from outside the host, because the network has the default gateway
mode "nat":

    	chain raw-PREROUTING {
    		type filter hook prerouting priority raw; policy accept;
    		ip daddr 192.0.2.2 iifname != "bridge1" counter drop comment "DROP DIRECT ACCESS"
    	}


The `nat-prerouting-and-output` chain has a rule to DNAT from host port 8080 to
the container's port 80:

    	chain nat-prerouting-and-output {
    		iifname != "bridge1" tcp dport 8080 counter dnat to 192.0.2.2:80 comment "DNAT"
    	}

