<!-- This is a generated file; DO NOT EDIT. -->

## Container on a routed-mode network, with a published port

Running the daemon with the userland proxy disabled then, as before, adding a network running a container with a mapped port, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  -o com.docker.network.bridge.gateway_mode_ipv4=routed \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

Most rules are the same as the [nat mode network][1]:

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
    
    	chain filter-forward-in__bridge1 {
    		ct state established,related counter accept
    		ip protocol icmp counter accept comment "ICMP"
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
    	}
    }
    

</details>

But ...

In chain raw-PREROUTING, there's no "DROP DIRECT ACCESS" rule, so
container can be accessed from outside the host:

    	chain raw-PREROUTING {
    		type filter hook prerouting priority raw; policy accept;
    	}


In the `filter-forward-in` chain, there's a rule to accept ICMP:

    	chain filter-forward-in__bridge1 {
    		ct state established,related counter accept
    		ip protocol icmp counter accept comment "ICMP"
    		iifname "bridge1" counter accept comment "ICC"
    		ip daddr 192.0.2.2 tcp dport 80 counter accept
    		counter drop comment "UNPUBLISHED PORT DROP"
    	}


There are no masquerade rules:

    	chain nat-prerouting-and-output {
    	}

    	chain nat-postrouting-out__bridge1 {
    	}


_And, the userland proxy won't be started for mapped ports._

[1]: usernet-portmap.md
