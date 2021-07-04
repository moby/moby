#!/usr/bin/env bash

while getopts ":s" opt; do
	case $opt in
		s)
			SSD="true"
			;;
	esac
done

SSD="${SSD:-false}"

# Required tools
DOCKER="${DOCKER:-docker}"
NSENTER="${NSENTER:-nsenter}"
BRIDGE="${BRIDGE:-bridge}"
IPTABLES="${IPTABLES:-iptables}"
IPVSADM="${IPVSADM:-ipvsadm}"
IP="${IP:-ip}"
SSDBIN="${SSDBIN:-ssd}"
JQ="${JQ:-jq}"

networks=0
containers=0
ip_overlap=0

NSDIR=/var/run/docker/netns

function die() {
	echo $*
	exit 1
}

function echo_and_run() {
	echo "#" "$@"
	eval $(printf '%q ' "$@") < /dev/stdout
}

function check_ip_overlap() {
	inspect=$1
	overlap=$(echo "$inspect_output" | grep "EndpointIP\|VIP" | cut -d':' -f2 | sort | uniq -c | grep -v "1 ")
	if [ ! -z "$overlap" ]; then
		echo -e "\n\n*** OVERLAP on Network ${networkID} ***"
		echo -e "${overlap} \n\n"
		((ip_overlap++))
	else
		echo "No overlap"
	fi
}

type -P ${DOCKER} > /dev/null || echo "This tool requires the docker binary"
type -P ${NSENTER} > /dev/null || echo "This tool requires nsenter"
type -P ${BRIDGE} > /dev/null || echo "This tool requires bridge"
type -P ${IPTABLES} > /dev/null || echo "This tool requires iptables"
type -P ${IPVSADM} > /dev/null || echo "This tool requires ipvsadm"
type -P ${IP} > /dev/null || echo "This tool requires ip"
type -P ${JQ} > /dev/null || echo "This tool requires jq"

if ${DOCKER} network inspect --help | grep -q -- --verbose; then
	NETINSPECT_VERBOSE_SUPPORT="--verbose"
else
	NETINSPECT_VERBOSE_SUPPORT=""
fi

echo "Host iptables"
echo_and_run ${IPTABLES} -w1 -n -v -L -t filter | grep -v '^$'
echo_and_run ${IPTABLES} -w1 -n -v -L -t nat | grep -v '^$'
echo_and_run ${IPTABLES} -w1 -n -v -L -t mangle | grep -v '^$'
printf "\n"

echo "Host links addresses and routes"
echo_and_run ${IP} -o link show
echo_and_run ${IP} -o -4 address show
echo_and_run ${IP} -4 route show
printf "\n"

echo "Overlay network configuration"
for networkID in $(${DOCKER} network ls --no-trunc --filter driver=overlay -q) "ingress_sbox"; do
	echo "nnn Network ${networkID}"
	if [ "${networkID}" != "ingress_sbox" ]; then
		nspath=($(ls ${NSDIR}/*${networkID:0:9}*))
		inspect_output=$(${DOCKER} network inspect ${NETINSPECT_VERBOSE_SUPPORT} ${networkID})
		echo "$inspect_output"
		check_ip_overlap $inspect_output
	else
		nspath=(${NSDIR}/${networkID})
	fi

	for i in "${nspath[@]}"; do
		echo_and_run ${NSENTER} --net=${i} ${IP} -o -4 address show
		echo_and_run ${NSENTER} --net=${i} ${IP} -4 route show
		echo_and_run ${NSENTER} --net=${i} ${IP} -4 neigh show
		bridges=$(${NSENTER} --net=${i} ${IP} -j link show type bridge | ${JQ} -r '.[].ifname')
		# break string to array
		bridges=(${bridges})
		for b in "${bridges[@]}"; do
			if [ -z ${b} ] || [ ${b} == "null" ]; then
				continue
			fi
			echo_and_run ${NSENTER} --net=${i} ${BRIDGE} fdb show br ${b}
		done
		echo_and_run ${NSENTER} --net=${i} ${IPTABLES} -w1 -n -v -L -t filter | grep -v '^$'
		echo_and_run ${NSENTER} --net=${i} ${IPTABLES} -w1 -n -v -L -t nat | grep -v '^$'
		echo_and_run ${NSENTER} --net=${i} ${IPTABLES} -w1 -n -v -L -t mangle | grep -v '^$'
		echo_and_run ${NSENTER} --net=${i} ${IPVSADM} -l -n
		printf "\n"
		((networks++))
	done
done

echo "Container network configuration"
while read containerID status; do
	echo "ccc Container ${containerID} state: ${status}"
	${DOCKER} container inspect ${containerID} --format 'Name:{{json .Name | printf "%s\n"}}Id:{{json .Id | printf "%s\n"}}Hostname:{{json .Config.Hostname | printf "%s\n"}}CreatedAt:{{json .Created | printf "%s\n"}}State:{{json .State|printf "%s\n"}}RestartCount:{{json .RestartCount | printf "%s\n" }}Labels:{{json .Config.Labels | printf "%s\n"}}NetworkSettings:{{json .NetworkSettings}}' | sed '/^State:/ {s/\\"/QUOTE/g; s/,"Output":"[^"]*"//g;}'
	if [ ${status} = "Up" ]; then
		nspath=$(docker container inspect --format {{.NetworkSettings.SandboxKey}} ${containerID})
		echo_and_run ${NSENTER} --net=${nspath[0]} ${IP} -o -4 address show
		echo_and_run ${NSENTER} --net=${nspath[0]} ${IP} -4 route show
		echo_and_run ${NSENTER} --net=${nspath[0]} ${IP} -4 neigh show
		echo_and_run ${NSENTER} --net=${nspath[0]} ${IPTABLES} -w1 -n -v -L -t nat | grep -v '^$'
		echo_and_run ${NSENTER} --net=${nspath[0]} ${IPTABLES} -w1 -n -v -L -t mangle | grep -v '^$'
		echo_and_run ${NSENTER} --net=${nspath[0]} ${IPVSADM} -l -n
		((containers++))
	fi
	printf "\n"
done < <(${DOCKER} container ls -a --format '{{.ID}} {{.Status}}' | cut -d' ' -f1,2)

if [ "true" == ${SSD} ]; then
	echo ""
	echo "#### SSD control-plane and datapath consistency check on a node ####"
	for netName in $(docker network ls -f driver=overlay --format "{{.Name}}"); do
		echo "## $netName ##"
		${SSDBIN} $netName
		echo ""
	done
fi

echo -e "\n\n==SUMMARY=="
echo -e "\t Processed $networks networks"
echo -e "\t IP overlap found: $ip_overlap"
echo -e "\t Processed $containers running containers"
