<!-- This is a generated file; DO NOT EDIT. -->

## Containers on user-defined --internal networks

These are the rules for two containers on different `--internal` networks, with and
without inter-container communication (ICC).

Equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridgeICC \
	  --internal \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridgeICC --name c1 busybox

	docker network create \
	  -o com.docker.network.bridge.name=bridgeNoICC \
	  -o com.docker.network.bridge.enable_icc=true \
	  --internal \
	  --subnet 198.51.100.0/24 --gateway 198.51.100.1 bridge1
	docker run --network bridgeNoICC --name c1 busybox

Most rules are the same as the network with [external access][0]:

<details>
<summary>Full table ...</summary>

    table ip docker-bridges {
    	map filter-forward-in-jumps {
    		type ifname : verdict
    		elements = { "docker0" : jump filter-forward-in__docker0,
    			     "bridgeICC" : jump filter-forward-in__bridgeICC,
    			     "bridgeNoICC" : jump filter-forward-in__bridgeNoICC }
    	}
    
    	map filter-forward-out-jumps {
    		type ifname : verdict
    		elements = { "docker0" : jump filter-forward-out__docker0,
    			     "bridgeICC" : jump filter-forward-out__bridgeICC,
    			     "bridgeNoICC" : jump filter-forward-out__bridgeNoICC }
    	}
    
    	map nat-postrouting-in-jumps {
    		type ifname : verdict
    		elements = { "docker0" : jump nat-postrouting-in__docker0,
    			     "bridgeICC" : jump nat-postrouting-in__bridgeICC,
    			     "bridgeNoICC" : jump nat-postrouting-in__bridgeNoICC }
    	}
    
    	map nat-postrouting-out-jumps {
    		type ifname : verdict
    		elements = { "docker0" : jump nat-postrouting-out__docker0,
    			     "bridgeICC" : jump nat-postrouting-out__bridgeICC,
    			     "bridgeNoICC" : jump nat-postrouting-out__bridgeNoICC }
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
    
    	chain filter-forward-in__bridgeICC {
    		ct state established,related counter accept
    		iifname != "bridgeICC" counter drop comment "INTERNAL NETWORK INGRESS"
    		counter accept comment "ICC"
    	}
    
    	chain filter-forward-out__bridgeICC {
    		ct state established,related counter accept
    		oifname != "bridgeICC" counter drop comment "INTERNAL NETWORK EGRESS"
    	}
    
    	chain nat-postrouting-in__bridgeICC {
    	}
    
    	chain nat-postrouting-out__bridgeICC {
    	}
    
    	chain filter-forward-in__bridgeNoICC {
    		ct state established,related counter accept
    		iifname != "bridgeNoICC" counter drop comment "INTERNAL NETWORK INGRESS"
    		counter drop comment "ICC"
    	}
    
    	chain filter-forward-out__bridgeNoICC {
    		ct state established,related counter accept
    		oifname != "bridgeNoICC" counter drop comment "INTERNAL NETWORK EGRESS"
    	}
    
    	chain nat-postrouting-in__bridgeNoICC {
    	}
    
    	chain nat-postrouting-out__bridgeNoICC {
    	}
    }
    

</details>

The filter-forward-in chains have rules to drop packets originating outside
the network. And, with ICC disabled, the final verdict is drop rather than
accept:

    	chain filter-forward-in__bridgeICC {
    		ct state established,related counter accept
    		iifname != "bridgeICC" counter drop comment "INTERNAL NETWORK INGRESS"
    		counter accept comment "ICC"
    	}

    	chain filter-forward-in__bridgeNoICC {
    		ct state established,related counter accept
    		iifname != "bridgeNoICC" counter drop comment "INTERNAL NETWORK INGRESS"
    		counter drop comment "ICC"
    	}


The nat-postrouting-out chains have no masquerade rules:

    	chain nat-postrouting-out__bridgeICC {
    	}

    	chain nat-postrouting-out__bridgeNoICC {
    	}


[0]: usernet-portmap.md
