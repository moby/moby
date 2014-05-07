#!/usr/bin/env bash
set -e

# bits of this were adapted from lxc-checkconfig
# see also https://github.com/lxc/lxc/blob/lxc-1.0.2/src/lxc/lxc-checkconfig.in

possibleConfigs=(
	'/proc/config.gz'
	"/boot/config-$(uname -r)"
	"/usr/src/linux-$(uname -r)/.config"
	'/usr/src/linux/.config'
)
: ${CONFIG:="${possibleConfigs[0]}"}

if ! command -v zgrep &> /dev/null; then
	zgrep() {
		zcat "$2" | grep "$1"
	}
fi

is_set() {
	zgrep "CONFIG_$1=[y|m]" "$CONFIG" > /dev/null
}

# see http://en.wikipedia.org/wiki/ANSI_escape_code#Colors
declare -A colors=(
	[black]=30
	[red]=31
	[green]=32
	[yellow]=33
	[blue]=34
	[magenta]=35
	[cyan]=36
	[white]=37
)
color() {
	color=()
	if [ "$1" = 'bold' ]; then
		color+=( '1' )
		shift
	fi
	if [ $# -gt 0 ] && [ "${colors[$1]}" ]; then
		color+=( "${colors[$1]}" )
	fi
	local IFS=';'
	echo -en '\033['"${color[*]}"m
}
wrap_color() {
	text="$1"
	shift
	color "$@"
	echo -n "$text"
	color reset
	echo
}

wrap_good() {
	echo "$(wrap_color "$1" white): $(wrap_color "$2" green)"
}
wrap_bad() {
	echo "$(wrap_color "$1" bold): $(wrap_color "$2" bold red)"
}
wrap_warning() {
	wrap_color >&2 "$*" red
}

check_flag() {
	if is_set "$1"; then
		wrap_good "CONFIG_$1" 'enabled'
	else
		wrap_bad "CONFIG_$1" 'missing'
	fi
}

check_flags() {
	for flag in "$@"; do
		echo "- $(check_flag "$flag")"
	done
} 

if [ ! -e "$CONFIG" ]; then
	wrap_warning "warning: $CONFIG does not exist, searching other paths for kernel config..."
	for tryConfig in "${possibleConfigs[@]}"; do
		if [ -e "$tryConfig" ]; then
			CONFIG="$tryConfig"
			break
		fi
	done
	if [ ! -e "$CONFIG" ]; then
		wrap_warning "error: cannot find kernel config"
		wrap_warning "  try running this script again, specifying the kernel config:"
		wrap_warning "    CONFIG=/path/to/kernel/.config $0"
		exit 1
	fi
fi

wrap_color "info: reading kernel config from $CONFIG ..." white
echo

echo 'Generally Necessary:'

echo -n '- '
cgroupSubsystemDir="$(awk '/[, ](cpu|cpuacct|cpuset|devices|freezer|memory)([, ]|$)/ && $8 == "cgroup" { print $5 }' /proc/$$/mountinfo | head -n1)"
cgroupDir="$(dirname "$cgroupSubsystemDir")"
if [ -d "$cgroupDir/cpu" -o -d "$cgroupDir/cpuacct" -o -d "$cgroupDir/cpuset" -o -d "$cgroupDir/devices" -o -d "$cgroupDir/freezer" -o -d "$cgroupDir/memory" ]; then
	echo "$(wrap_good 'cgroup hierarchy' 'properly mounted') [$cgroupDir]"
else
	if [ "$cgroupSubsystemDir" ]; then
		echo "$(wrap_bad 'cgroup hierarchy' 'single mountpoint!') [$cgroupSubsystemDir]"
	else
		echo "$(wrap_bad 'cgroup hierarchy' 'nonexistent??')"
	fi
	echo "    $(wrap_color '(see https://github.com/tianon/cgroupfs-mount)' yellow)"
fi

flags=(
	NAMESPACES {NET,PID,IPC,UTS}_NS
	DEVPTS_MULTIPLE_INSTANCES
	CGROUPS CGROUP_DEVICE
	MACVLAN VETH BRIDGE
	NF_NAT_IPV4 IP_NF_TARGET_MASQUERADE
	NETFILTER_XT_MATCH_{ADDRTYPE,CONNTRACK}
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
	echo '- "'$(wrap_color 'aufs' blue)'":'
	check_flags AUFS_FS | sed 's/^/  /'
	if ! is_set AUFS_FS && grep -q aufs /proc/filesystems; then
		echo "    $(wrap_color '(note that some kernels include AUFS patches but not the AUFS_FS flag)' bold black)"
	fi

	echo '- "'$(wrap_color 'btrfs' blue)'":'
	check_flags BTRFS_FS | sed 's/^/  /'

	echo '- "'$(wrap_color 'devicemapper' blue)'":'
	check_flags BLK_DEV_DM DM_THIN_PROVISIONING EXT4_FS | sed 's/^/  /'
} | sed 's/^/  /'
echo

#echo 'Potential Future Features:'
#check_flags USER_NS
#echo
