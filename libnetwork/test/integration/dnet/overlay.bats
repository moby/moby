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
	osvc="svc$i"
	dnet_cmd $(inst_id2port $i) service publish ${osvc}.multihost
	dnet_cmd $(inst_id2port $i) container create container_${i}
	dnet_cmd $(inst_id2port $i) service attach container_${i} ${osvc}.multihost
    done

    # Now test connectivity between all the containers using service names
    for i in `seq ${start} ${end}`;
    do
	src="svc$i"
	line=$(dnet_cmd $(inst_id2port $i) service ls | grep ${src})
	echo ${line}
	sbid=$(echo ${line} | cut -d" " -f5)
	for j in `seq ${start} ${end}`;
	do
	    if [ "$i" -eq "$j" ]; then
		continue
	    fi
	    runc $(dnet_container_name $i overlay) ${sbid} "ping -c 1 svc$j"
	done
    done

    # Teardown the container connections and the network
    for i in `seq ${start} ${end}`;
    do
	osvc="svc$i"
	dnet_cmd $(inst_id2port $i) service detach container_${i} ${osvc}.multihost
	dnet_cmd $(inst_id2port $i) container rm container_${i}
	dnet_cmd $(inst_id2port $i) service unpublish ${osvc}.multihost
    done

    run dnet_cmd $(inst_id2port 2) network rm multihost
}
