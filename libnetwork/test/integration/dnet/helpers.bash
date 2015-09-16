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
    stop_dnet $1 $2
    name=$(dnet_container_name $1 $2)
    if [ -z "$3" ]
    then
	hport=$((41000+${1}-1))
	cport=2385
	hopt=""
    else
	hport=$3
	cport=$3
	hopt="-H tcp://0.0.0.0:${cport}"
    fi

    bridge_ip=$(docker inspect --format '{{.NetworkSettings.Gateway}}' pr_consul)
    mkdir -p /tmp/dnet/${name}
    tomlfile="/tmp/dnet/${name}/libnetwork.toml"
    cat > ${tomlfile} <<EOF
title = "LibNetwork Configuration file"

[daemon]
  debug = false
  labels = ["com.docker.network.driver.overlay.bind_interface=eth0"]
[globalstore]
  embedded = false
[globalstore.client]
  provider = "consul"
  Address = "${bridge_ip}:8500"
EOF
    docker run \
	   -d \
	   --name=${name}  \
	   --privileged \
	   -p ${hport}:${cport} \
	   -v $(pwd)/:/go/src/github.com/docker/libnetwork \
	   -v /tmp:/tmp \
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
