#!/usr/bin/env bash

# Required tools
DOCKER="${DOCKER:-docker}"
NSENTER="${NSENTER:-nsenter}"
BRIDGE="${BRIDGE:-bridge}"
IPTABLES="${IPTABLES:-iptables}"
IPVSADM="${IPVSADM:-ipvsadm}"
IP="${IP:-ip}"

NSDIR=/var/run/docker/netns

function die {
    echo $*
    exit 1
}

function echo_and_run {
  echo "#" "$@"
  eval $(printf '%q ' "$@") < /dev/stdout
}

type -P ${DOCKER} > /dev/null || echo "This tool requires the docker binary"
type -P ${NSENTER} > /dev/null || echo "This tool requires nsenter"
type -P ${BRIDGE} > /dev/null || echo "This tool requires bridge"
type -P ${IPTABLES} > /dev/null || echo "This tool requires iptables"
type -P ${IPVSADM} > /dev/null || echo "This tool requires ipvsadm"
type -P ${IP} > /dev/null || echo "This tool requires ip"

if ${DOCKER} network inspect --help | grep -q -- --verbose; then
    NETINSPECT_VERBOSE_SUPPORT="--verbose"
else
    NETINSPECT_VERBOSE_SUPPORT=""
fi

echo "Host Configuration"
echo_and_run ${IPTABLES} -w1 -n -v -L -t filter | grep -v '^$'
echo_and_run ${IPTABLES} -w1 -n -v -L -t nat | grep -v '^$'
echo_and_run ${IPTABLES} -w1 -n -v -L -t mangle | grep -v '^$'
printf "\n"

echo "Host addresses and routes"
echo_and_run ${IP} -o -4 address show
echo_and_run ${IP} -4 route show
printf "\n"

echo "Overlay network configuration"
for networkID in $(${DOCKER} network ls --no-trunc --filter driver=overlay -q) "ingress_sbox"; do
    echo "nnn Network ${networkID}"
    if [ "${networkID}" != "ingress_sbox" ]; then
        nspath=(${NSDIR}/*-${networkID:0:10})
        ${DOCKER} network inspect ${NETINSPECT_VERBOSE_SUPPORT} ${networkID}
    else
        nspath=(${NSDIR}/${networkID})
    fi
    echo_and_run ${NSENTER} --net=${nspath[0]} ${IP} -o -4 address show
    echo_and_run ${NSENTER} --net=${nspath[0]} ${IP} -4 route show
    echo_and_run ${NSENTER} --net=${nspath[0]} ${IP} -4 neigh show
    echo_and_run ${NSENTER} --net=${nspath[0]} ${BRIDGE} fdb show
    echo_and_run ${NSENTER} --net=${nspath[0]} ${IPTABLES} -w1 -n -v -L -t filter | grep -v '^$'
    echo_and_run ${NSENTER} --net=${nspath[0]} ${IPTABLES} -w1 -n -v -L -t nat | grep -v '^$'
    echo_and_run ${NSENTER} --net=${nspath[0]} ${IPTABLES} -w1 -n -v -L -t mangle | grep -v '^$'
    echo_and_run ${NSENTER} --net=${nspath[0]} ${IPVSADM} -l -n
    printf "\n"
done

echo "Container network configuration"
for containerID in $(${DOCKER} container ls -q); do
    echo "ccc Container ${containerID}"
    ${DOCKER} container inspect ${containerID} --format 'Name:{{json .Name | printf "%s\n"}}Id:{{json .Id | printf "%s\n"}}Hostname:{{json .Config.Hostname | printf "%s\n"}}CreatedAt:{{json .Created | printf "%s\n"}}State:{{json .State|printf "%s\n"}}RestartCount:{{json .RestartCount | printf "%s\n" }}Labels:{{json .Config.Labels | printf "%s\n"}}NetworkSettings:{{json .NetworkSettings}}' | sed '/^State:/ {s/\\"/QUOTE/g; s/,"Output":"[^"]*"//g;}'
    nspath=$(docker container inspect --format {{.NetworkSettings.SandboxKey}} ${containerID})
    echo_and_run ${NSENTER} --net=${nspath[0]} ${IP} -o -4 address show
    echo_and_run ${NSENTER} --net=${nspath[0]} ${IP} -4 route show
    echo_and_run ${NSENTER} --net=${nspath[0]} ${IP} -4 neigh show
    echo_and_run ${NSENTER} --net=${nspath[0]} ${IPTABLES} -w1 -n -v -L -t nat | grep -v '^$'
    echo_and_run ${NSENTER} --net=${nspath[0]} ${IPTABLES} -w1 -n -v -L -t mangle | grep -v '^$'
    echo_and_run ${NSENTER} --net=${nspath[0]} ${IPVSADM} -l -n
    printf "\n"
done
