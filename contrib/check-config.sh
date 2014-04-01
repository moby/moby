#!/usr/bin/env bash
set -e

# bits of this were adapted from lxc-checkconfig
# see also https://github.com/lxc/lxc/blob/lxc-1.0.2/src/lxc/lxc-checkconfig.in

: ${CONFIG:=/proc/config.gz}
: ${GREP:=zgrep}

if [ ! -e "$CONFIG" ]; then
	echo >&2 "warning: $CONFIG does not exist, searching other paths for kernel config..."
	if [ -e "/boot/config-$(uname -r)" ]; then
		CONFIG="/boot/config-$(uname -r)"
	elif [ -e '/usr/src/linux/.config' ]; then
		CONFIG='/usr/src/linux/.config'
	else
		echo >&2 "error: cannot find kernel config"
		echo >&2 "  try running this script again, specifying the kernel config:"
		echo >&2 "    CONFIG=/path/to/kernel/.config $0"
		exit 1
	fi
fi

is_set() {
	$GREP "CONFIG_$1=[y|m]" $CONFIG > /dev/null
}

color() {
	color=
	prefix=
	if [ "$1" = 'bold' ]; then
		prefix='1;'
		shift
	fi
	case "$1" in
		green) color='32' ;;
		red)   color='31' ;;
		gray)  color='30' ;;
		reset) color='' ;;
	esac
	echo -en '\033['"$prefix$color"m
}

check_flag() {
	if is_set "$1"; then
		color green
		echo -n enabled
	else
		color bold red
		echo -n missing
	fi
	color reset
}

check_flags() {
	for flag in "$@"; do
		echo "- CONFIG_$flag: $(check_flag "$flag")"
	done
} 

echo

# TODO check that the cgroupfs hierarchy is properly mounted

echo 'Generally Necessary:'
flags=(
	NAMESPACES {NET,PID,IPC,UTS}_NS
	DEVPTS_MULTIPLE_INSTANCES
	CGROUPS CGROUP_DEVICE
	MACVLAN VETH BRIDGE
	IP_NF_TARGET_MASQUERADE NETFILTER_XT_MATCH_{ADDRTYPE,CONNTRACK}
	NF_NAT NF_NAT_NEEDED
)
check_flags "${flags[@]}"
echo

echo 'Optional Features:'
flags=(
	MEMCG_SWAP
	RESOURCE_COUNTERS
)
check_flags "${flags[@]}"

echo '- Storage Drivers:'
{
	echo '- "aufs":'
	check_flags AUFS_FS | sed 's/^/  /'
	if ! is_set AUFS_FS && grep -q aufs /proc/filesystems; then
		echo "    $(color bold gray)(note that some kernels include AUFS patches but not the AUFS_FS flag)$(color reset)"
	fi

	echo '- "btrfs":'
	check_flags BTRFS_FS | sed 's/^/  /'

	echo '- "devicemapper":'
	check_flags BLK_DEV_DM DM_THIN_PROVISIONING EXT4_FS | sed 's/^/  /'
} | sed 's/^/  /'
echo

#echo 'Potential Future Features:'
#check_flags USER_NS
#echo
