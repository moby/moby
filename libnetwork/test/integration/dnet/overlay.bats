# -*- mode: sh -*-
#!/usr/bin/env bats

load helpers

@test "Test overlay network" {
    skip_for_circleci

    echo $(docker ps)

    start=1
    end=3
    # Setup overlay network and connect containers ot it
    dnet_cmd $(inst_id2port 1) network create -d overlay multihost
    for i in `seq ${start} ${end}`;
    do
	dnet_cmd $(inst_id2port $i) container create container_${i}
	net_connect ${i} container_${i} multihost
    done

    # Now test connectivity between all the containers using service names
    for i in `seq ${start} ${end}`;
    do
	for j in `seq ${start} ${end}`;
	do
	    if [ "$i" -eq "$j" ]; then
		continue
	    fi
	    runc $(dnet_container_name $i overlay) $(get_sbox_id ${i} container_${i}) \
		 "ping -c 1 container_$j"
	done
    done

    # Teardown the container connections and the network
    for i in `seq ${start} ${end}`;
    do
	net_disconnect ${i} container_${i} multihost
	dnet_cmd $(inst_id2port $i) container rm container_${i}
    done

    run dnet_cmd $(inst_id2port 2) network rm multihost
}
