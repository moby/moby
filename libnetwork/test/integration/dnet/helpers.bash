function inst_id2port() {
    echo $((41000+${1}-1))
}

function dnet_container_name() {
    echo dnet-$1-$2
}

function get_sbox_id() {
    local line

    line=$(dnet_cmd $(inst_id2port ${1}) service ls | grep ${2})
    echo ${line} | cut -d" " -f5
}

function net_connect() {
	dnet_cmd $(inst_id2port ${1}) service publish ${2}.${3}
	dnet_cmd $(inst_id2port ${1}) service attach ${2} ${2}.${3}
}

function net_disconnect() {
	dnet_cmd $(inst_id2port ${1}) service detach ${2} ${2}.${3}
	dnet_cmd $(inst_id2port ${1}) service unpublish ${2}.${3}
}

function start_consul() {
    stop_consul
    docker run -d \
	   --name=pr_consul \
	   -p 8500:8500 \
	   -p 8300-8302:8300-8302/tcp \
	   -p 8300-8302:8300-8302/udp \
	   -h consul \
	   progrium/consul -server -bootstrap
    sleep 2
}

function stop_consul() {
    echo "consul started"
    docker stop pr_consul || true
    # You cannot destroy a container in Circle CI. So do not attempt destroy in circleci
    if [ -z "$CIRCLECI" ]; then
	docker rm -f pr_consul || true
    fi
}

hrun() {
    local e E T oldIFS
    [[ ! "$-" =~ e ]] || e=1
    [[ ! "$-" =~ E ]] || E=1
    [[ ! "$-" =~ T ]] || T=1
    set +e
    set +E
    set +T
    output="$("$@" 2>&1)"
    status="$?"
    oldIFS=$IFS
    IFS=$'\n' lines=($output)
    [ -z "$e" ] || set -e
    [ -z "$E" ] || set -E
    [ -z "$T" ] || set -T
    IFS=$oldIFS
}

function wait_for_dnet() {
    local hport

    hport=$1
    echo "waiting on dnet to come up ..."
    for i in `seq 1 10`;
    do
	hrun ./cmd/dnet/dnet -H tcp://127.0.0.1:${hport} network ls
	echo ${output}
	if [ "$status" -eq 0 ]; then
	    return
	fi

	if [[ "${lines[1]}" =~ .*EOF.* ]]
	then
	    docker logs ${2}
	fi
	echo "still waiting after ${i} seconds"
	sleep 1
    done
}

function start_dnet() {
    local inst suffix name hport cport hopt neighip bridge_ip labels tomlfile
    inst=$1
    shift
    suffix=$1
    shift

    stop_dnet ${inst} ${suffix}
    name=$(dnet_container_name ${inst} ${suffix})

    hport=$((41000+${inst}-1))
    cport=2385
    hopt=""

    while [ -n "$1" ]
    do
	if [[ "$1" =~ ^[0-9]+$ ]]
	then
	    hport=$1
	    cport=$1
	    hopt="-H tcp://0.0.0.0:${cport}"
	else
	    neighip=$1
	fi
	shift
    done

    bridge_ip=$(docker inspect --format '{{.NetworkSettings.Gateway}}' pr_consul)

    if [ -z "$neighip" ]; then
	labels="\"com.docker.network.driver.overlay.bind_interface=eth0\""
    else
	labels="\"com.docker.network.driver.overlay.bind_interface=eth0\", \"com.docker.network.driver.overlay.neighbor_ip=${neighip}\""
    fi

    echo "parsed values: " ${name} ${hport} ${cport} ${hopt} ${neighip} ${labels}

    mkdir -p /tmp/dnet/${name}
    tomlfile="/tmp/dnet/${name}/libnetwork.toml"
    cat > ${tomlfile} <<EOF
title = "LibNetwork Configuration file for ${name}"

[daemon]
  debug = false
  labels = [${labels}]
[cluster]
  discovery = "consul://${bridge_ip}:8500"
  Heartbeat = 10
[scopes]
  [scopes.global]
    embedded = false
    [scopes.global.client]
      provider = "consul"
      address = "${bridge_ip}:8500"
EOF
    docker run \
	   -d \
	   --name=${name}  \
	   --privileged \
	   -p ${hport}:${cport} \
	   -v $(pwd)/:/go/src/github.com/docker/libnetwork \
	   -v /tmp:/tmp \
	   -v $(pwd)/${TMPC_ROOT}:/scratch \
	   -v /usr/local/bin/runc:/usr/local/bin/runc \
	   -w /go/src/github.com/docker/libnetwork \
	   golang:1.4 ./cmd/dnet/dnet -d -D ${hopt} -c ${tomlfile}
    wait_for_dnet $(inst_id2port ${inst}) ${name}
}

function skip_for_circleci() {
    if [ -n "$CIRCLECI" ]; then
	skip
    fi
}

function stop_dnet() {
    local name

    name=$(dnet_container_name $1 $2)
    rm -rf /tmp/dnet/${name} || true
    docker stop ${name} || true
    # You cannot destroy a container in Circle CI. So do not attempt destroy in circleci
    if [ -z "$CIRCLECI" ]; then
	docker rm -f ${name} || true
    fi
}

function dnet_cmd() {
    local hport

    hport=$1
    shift
    ./cmd/dnet/dnet -H tcp://127.0.0.1:${hport} $*
}

function dnet_exec() {
    docker exec -it ${1} bash -c "$2"
}

function runc() {
    local dnet

    dnet=${1}
    shift
    dnet_exec ${dnet} "cp /var/lib/docker/network/files/${1}*/* /scratch/rootfs/etc"
    dnet_exec ${dnet} "mkdir -p /var/run/netns"
    dnet_exec ${dnet} "touch /var/run/netns/c && mount -o bind /var/run/docker/netns/${1} /var/run/netns/c"
    dnet_exec ${dnet} "ip netns exec c unshare -fmuip --mount-proc chroot \"/scratch/rootfs\" /bin/sh -c \"/bin/mount -t proc proc /proc && ${2}\""
    dnet_exec ${dnet} "umount /var/run/netns/c && rm /var/run/netns/c"
}
