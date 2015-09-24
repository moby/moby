function inst_id2port() {
    echo $((41000+${1}-1))
}

function dnet_container_name() {
    echo dnet-$1-$2
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

function start_dnet() {
    inst=$1
    shift
    suffix=$1
    shift

    stop_dnet ${inst} ${suffix}
    name=$(dnet_container_name ${inst} ${suffix})

    hport=$((41000+${inst}-1))
    cport=2385
    hopt=""
    isnum='^[0-9]+$'

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

    mkdir -p /tmp/dnet/${name}
    tomlfile="/tmp/dnet/${name}/libnetwork.toml"
    cat > ${tomlfile} <<EOF
title = "LibNetwork Configuration file"

[daemon]
  debug = false
  labels = [${labels}]
[globalstore]
  embedded = false
[globalstore.client]
  provider = "consul"
  Address = "${bridge_ip}:8500"
EOF
    echo "parsed values: " ${name} ${hport} ${cport} ${hopt}
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
    sleep 2
}

function skip_for_circleci() {
    if [ -n "$CIRCLECI" ]; then
	skip
    fi
}

function stop_dnet() {
    name=$(dnet_container_name $1 $2)
    rm -rf /tmp/dnet/${name} || true
    docker stop ${name} || true
    # You cannot destroy a container in Circle CI. So do not attempt destroy in circleci
    if [ -z "$CIRCLECI" ]; then
	docker rm -f ${name} || true
    fi
}

function dnet_cmd() {
    hport=$1
    shift
    ./cmd/dnet/dnet -H tcp://127.0.0.1:${hport} $*
}

function dnet_exec() {
    docker exec -it ${1} bash -c "$2"
}

function runc() {
    dnet=${1}
    shift
    dnet_exec ${dnet} "cp /var/lib/docker/network/files/${1}*/* /scratch/rootfs/etc"
    dnet_exec ${dnet} "mkdir -p /var/run/netns"
    dnet_exec ${dnet} "touch /var/run/netns/c && mount -o bind /var/run/docker/netns/${1} /var/run/netns/c"
    dnet_exec ${dnet} "ip netns exec c unshare -fmuip --mount-proc chroot \"/scratch/rootfs\" /bin/sh -c \"/bin/mount -t proc proc /proc && ${2}\""
    dnet_exec ${dnet} "umount /var/run/netns/c && rm /var/run/netns/c"
}
