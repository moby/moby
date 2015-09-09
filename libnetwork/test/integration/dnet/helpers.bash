function start_consul() {
    docker run -d --name=pr_consul -p 8500:8500 -p 8300-8302:8300-8302/tcp -p 8300-8302:8300-8302/udp -h consul progrium/consul -server -bootstrap
    sleep 2
}

function stop_consul() {
    docker stop pr_consul
    # You cannot destroy a container in Circle CI. So do not attempt destroy in circleci
    if [ -z "$CIRCLECI" ]; then
	docker rm pr_consul
    fi
}

function start_dnet() {
    name="dnet-$1"
    hport=$((41000+${1}-1))

    bridge_ip=$(docker inspect --format '{{.NetworkSettings.Gateway}}' pr_consul)
    mkdir -p /tmp/dnet/${name}
    tomlfile="/tmp/dnet/${name}/libnetwork.toml"
    cat > ${tomlfile} <<EOF
title = "LibNetwork Configuration file"

[daemon]
  debug = false
  defaultnetwork = "${2}"
  defaultdriver = "${3}"
  labels = ["com.docker.network.driver.overlay.bind_interface=eth0"]
[datastore]
  embedded = false
[datastore.client]
  provider = "consul"
  Address = "${bridge_ip}:8500"
EOF
    docker run -d --name=${name}  --privileged -p ${hport}:2385 -v $(pwd)/:/go/src/github.com/docker/libnetwork -v /tmp:/tmp -w /go/src/github.com/docker/libnetwork golang:1.4 ./cmd/dnet/dnet -dD -c ${tomlfile}
    sleep 2
}

function stop_dnet() {
    name="dnet-$1"
    rm -rf /tmp/dnet/${name}
    docker stop ${name}
    # You cannot destroy a container in Circle CI. So do not attempt destroy in circleci
    if [ -z "$CIRCLECI" ]; then
	docker rm ${name} || true
    fi

}

function dnet_cmd() {
    hport=$((41000+${1}-1))
    shift
    ./cmd/dnet/dnet -H 127.0.0.1:${hport} $*
}
