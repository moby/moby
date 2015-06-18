#!/bin/bash
#
# bash completion file for core docker commands
#
# This script provides completion of:
#  - commands and their options
#  - container ids and names
#  - image repos and tags
#  - filepaths
#
# To enable the completions either:
#  - place this file in /etc/bash_completion.d
#  or
#  - copy this file to e.g. ~/.docker-completion.sh and add the line
#    below to your .bashrc after bash completion features are loaded
#    . ~/.docker-completion.sh
#
# Note:
# Currently, the completions will not work if the docker daemon is not
# bound to the default communication port/socket
# If the docker daemon is using a unix socket for communication your user
# must have access to the socket for the completions to function correctly
#
# Note for developers:
# Please arrange options sorted alphabetically by long name with the short
# options immediately following their corresponding long form.
# This order should be applied to lists, alternatives and code blocks.

__docker_q() {
	docker ${host:+-H "$host"} 2>/dev/null "$@"
}

__docker_containers_all() {
	local IFS=$'\n'
	local containers=( $(__docker_q ps -aq --no-trunc) )
	if [ "$1" ]; then
		containers=( $(__docker_q inspect --format "{{if $1}}{{.Id}}{{end}}" "${containers[@]}") )
	fi
	local names=( $(__docker_q inspect --format '{{.Name}}' "${containers[@]}") )
	names=( "${names[@]#/}" ) # trim off the leading "/" from the container names
	unset IFS
	COMPREPLY=( $(compgen -W "${names[*]} ${containers[*]}" -- "$cur") )
}

__docker_containers_running() {
	__docker_containers_all '.State.Running'
}

__docker_containers_stopped() {
	__docker_containers_all 'not .State.Running'
}

__docker_containers_pauseable() {
	__docker_containers_all 'and .State.Running (not .State.Paused)'
}

__docker_containers_unpauseable() {
	__docker_containers_all '.State.Paused'
}

__docker_container_names() {
	local containers=( $(__docker_q ps -aq --no-trunc) )
	local names=( $(__docker_q inspect --format '{{.Name}}' "${containers[@]}") )
	names=( "${names[@]#/}" ) # trim off the leading "/" from the container names
	COMPREPLY=( $(compgen -W "${names[*]}" -- "$cur") )
}

__docker_container_ids() {
	local containers=( $(__docker_q ps -aq) )
	COMPREPLY=( $(compgen -W "${containers[*]}" -- "$cur") )
}

__docker_image_repos() {
	local repos="$(__docker_q images | awk 'NR>1 && $1 != "<none>" { print $1 }')"
	COMPREPLY=( $(compgen -W "$repos" -- "$cur") )
}

__docker_image_repos_and_tags() {
	local reposAndTags="$(__docker_q images | awk 'NR>1 && $1 != "<none>" { print $1; print $1":"$2 }')"
	COMPREPLY=( $(compgen -W "$reposAndTags" -- "$cur") )
	__ltrim_colon_completions "$cur"
}

__docker_image_repos_and_tags_and_ids() {
	local images="$(__docker_q images -a --no-trunc | awk 'NR>1 { print $3; if ($1 != "<none>") { print $1; print $1":"$2 } }')"
	COMPREPLY=( $(compgen -W "$images" -- "$cur") )
	__ltrim_colon_completions "$cur"
}

__docker_containers_and_images() {
	__docker_containers_all
	local containers=( "${COMPREPLY[@]}" )
	__docker_image_repos_and_tags_and_ids
	COMPREPLY+=( "${containers[@]}" )
}

__docker_pos_first_nonflag() {
	local argument_flags=$1

	local counter=$cpos
	while [ $counter -le $cword ]; do
		if [ -n "$argument_flags" ] && eval "case '${words[$counter]}' in $argument_flags) true ;; *) false ;; esac"; then
			(( counter++ ))
		else
			case "${words[$counter]}" in
				-*)
					;;
				*)
					break
					;;
			esac
		fi
		(( counter++ ))
	done

	echo $counter
}

# Transforms a multiline list of strings into a single line string
# with the words separated by "|".
# This is used to prepare arguments to __docker_pos_first_nonflag().
__docker_to_alternatives() {
	local parts=( $1 )
	local IFS='|'
	echo "${parts[*]}"
}

# Transforms a multiline list of options into an extglob pattern
# suitable for use in case statements.
__docker_to_extglob() {
	local extglob=$( __docker_to_alternatives "$1" )
	echo "@($extglob)"
}

__docker_resolve_hostname() {
	command -v host >/dev/null 2>&1 || return
	COMPREPLY=( $(host 2>/dev/null "${cur%:}" | awk '/has address/ {print $4}') )
}

__docker_capabilities() {
	# The list of capabilities is defined in types.go, ALL was added manually.
	COMPREPLY=( $( compgen -W "
		ALL
		AUDIT_CONTROL
		AUDIT_WRITE
		AUDIT_READ
		BLOCK_SUSPEND
		CHOWN
		DAC_OVERRIDE
		DAC_READ_SEARCH
		FOWNER
		FSETID
		IPC_LOCK
		IPC_OWNER
		KILL
		LEASE
		LINUX_IMMUTABLE
		MAC_ADMIN
		MAC_OVERRIDE
		MKNOD
		NET_ADMIN
		NET_BIND_SERVICE
		NET_BROADCAST
		NET_RAW
		SETFCAP
		SETGID
		SETPCAP
		SETUID
		SYS_ADMIN
		SYS_BOOT
		SYS_CHROOT
		SYSLOG
		SYS_MODULE
		SYS_NICE
		SYS_PACCT
		SYS_PTRACE
		SYS_RAWIO
		SYS_RESOURCE
		SYS_TIME
		SYS_TTY_CONFIG
		WAKE_ALARM
	" -- "$cur" ) )
}

# a selection of the available signals that is most likely of interest in the
# context of docker containers.
__docker_signals() {
	local signals=(
		SIGCONT
		SIGHUP
		SIGINT
		SIGKILL
		SIGQUIT
		SIGSTOP
		SIGTERM
		SIGUSR1
		SIGUSR2
	)
	COMPREPLY=( $( compgen -W "${signals[*]} ${signals[*]#SIG}" -- "$( echo $cur | tr '[:lower:]' '[:upper:]')" ) )
}

_docker_docker() {
	local boolean_options="
		--daemon -d
		--debug -D
		--help -h
		--icc
		--ip-forward
		--ip-masq
		--iptables
		--ipv6
		--selinux-enabled
		--tls
		--tlsverify
		--userland-proxy=false
		--version -v
	"

	case "$prev" in
		--exec-root|--graph|-g)
			_filedir -d
			return
			;;
		--log-driver)
			COMPREPLY=( $( compgen -W "json-file syslog none" -- "$cur" ) )
			return
			;;
		--log-level|-l)
			COMPREPLY=( $( compgen -W "debug info warn error fatal" -- "$cur" ) )
			return
			;;
		--pidfile|-p|--tlscacert|--tlscert|--tlskey)
			_filedir
			return
			;;
		--storage-driver|-s)
			COMPREPLY=( $( compgen -W "aufs devicemapper btrfs overlay" -- "$(echo $cur | tr '[:upper:]' '[:lower:]')" ) )
			return
			;;
		$main_options_with_args_glob )
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "$boolean_options $main_options_with_args" -- "$cur" ) )
			;;
		*)
			COMPREPLY=( $( compgen -W "${commands[*]} help" -- "$cur" ) )
			;;
	esac
}

_docker_attach() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help --no-stdin --sig-proxy" -- "$cur" ) )
			;;
		*)
			local counter="$(__docker_pos_first_nonflag)"
			if [ $cword -eq $counter ]; then
				__docker_containers_running
			fi
			;;
	esac
}

_docker_build() {
	case "$prev" in
		--cgroup-parent|--cpuset-cpus|--cpuset-mems|--cpu-shares|-c|--cpu-period|--cpu-quota|--memory|-m|--memory-swap)
			return
			;;
		--file|-f)
			_filedir
			return
			;;
		--tag|-t)
			__docker_image_repos_and_tags
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--cgroup-parent --cpuset-cpus --cpuset-mems --cpu-shares -c --cpu-period --cpu-quota --file -f --force-rm --help --memory -m --memory-swap --no-cache --pull --quiet -q --rm --tag -t" -- "$cur" ) )
			;;
		*)
			local counter="$(__docker_pos_first_nonflag '--cgroup-parent|--cpuset-cpus|--cpuset-mems|--cpu-shares|-c|--cpu-period|--cpu-quota|--file|-f|--memory|-m|--memory-swap|--tag|-t')"
			if [ $cword -eq $counter ]; then
				_filedir -d
			fi
			;;
	esac
}

_docker_commit() {
	case "$prev" in
		--author|-a|--change|-c|--message|-m)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--author -a --change -c --help --message -m --pause -p" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag '--author|-a|--change|-c|--message|-m')

			if [ $cword -eq $counter ]; then
				__docker_containers_all
				return
			fi
			(( counter++ ))

			if [ $cword -eq $counter ]; then
				__docker_image_repos_and_tags
				return
			fi
			;;
	esac
}

_docker_cp() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				case "$cur" in
					*:)
						return
						;;
					*)
						__docker_containers_all
						COMPREPLY=( $( compgen -W "${COMPREPLY[*]}" -S ':' ) )
						compopt -o nospace
						return
						;;
				esac
			fi
			(( counter++ ))

			if [ $cword -eq $counter ]; then
				_filedir -d
				return
			fi
			;;
	esac
}

_docker_create() {
	_docker_run
}

_docker_diff() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				__docker_containers_all
			fi
			;;
	esac
}

_docker_events() {
	case "$prev" in
		--filter|-f)
			COMPREPLY=( $( compgen -S = -W "container event image" -- "$cur" ) )
			compopt -o nospace
			return
			;;
		--since|--until)
			return
			;;
	esac

	# "=" gets parsed to a word and assigned to either $cur or $prev depending on whether
	# it is the last character or not. So we search for "xxx=" in the the last two words.
	case "${words[$cword-2]}$prev=" in
		*container=*)
			cur="${cur#=}"
			__docker_containers_all
			return
			;;
		*event=*)
			COMPREPLY=( $( compgen -W "create destroy die export kill pause restart start stop unpause" -- "${cur#=}" ) )
			return
			;;
		*image=*)
			cur="${cur#=}"
			__docker_image_repos_and_tags_and_ids
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--filter -f --help --since --until" -- "$cur" ) )
			;;
	esac
}

_docker_exec() {
	case "$prev" in
		--user|-u)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--detach -d --help --interactive -i -t --tty -u --user" -- "$cur" ) )
			;;
		*)
			__docker_containers_running
			;;
	esac
}

_docker_export() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				__docker_containers_all
			fi
			;;
	esac
}

_docker_help() {
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		COMPREPLY=( $( compgen -W "${commands[*]}" -- "$cur" ) )
	fi
}

_docker_history() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help --no-trunc --quiet -q" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				__docker_image_repos_and_tags_and_ids
			fi
			;;
	esac
}

_docker_images() {
	case "$prev" in
		--filter|-f)
			COMPREPLY=( $( compgen -W "dangling=true label=" -- "$cur" ) )
			if [ "$COMPREPLY" = "label=" ]; then
				compopt -o nospace
			fi
			return
			;;
	esac

	case "${words[$cword-2]}$prev=" in
		*dangling=*)
			COMPREPLY=( $( compgen -W "true false" -- "${cur#=}" ) )
			return
			;;
		*label=*)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--all -a --digests --filter -f --help --no-trunc --quiet -q" -- "$cur" ) )
			;;
		=)
			return
			;;
		*)
			__docker_image_repos
			;;
	esac
}

_docker_import() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				return
			fi
			(( counter++ ))

			if [ $cword -eq $counter ]; then
				__docker_image_repos_and_tags
				return
			fi
			;;
	esac
}

_docker_info() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
	esac
}

_docker_inspect() {
	case "$prev" in
		--format|-f)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--format -f --help" -- "$cur" ) )
			;;
		*)
			__docker_containers_and_images
			;;
	esac
}

_docker_kill() {
	case "$prev" in
		--signal|-s)
			__docker_signals
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help --signal -s" -- "$cur" ) )
			;;
		*)
			__docker_containers_running
			;;
	esac
}

_docker_load() {
	case "$prev" in
		--input|-i)
			_filedir
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help --input -i" -- "$cur" ) )
			;;
	esac
}

_docker_login() {
	case "$prev" in
		--email|-e|--password|-p|--username|-u)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--email -e --help --password -p --username -u" -- "$cur" ) )
			;;
	esac
}

_docker_logout() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
	esac
}

_docker_logs() {
	case "$prev" in
		--since|--tail)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--follow -f --help --since --tail --timestamps -t" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag '--tail')
			if [ $cword -eq $counter ]; then
				__docker_containers_all
			fi
			;;
	esac
}

_docker_pause() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				__docker_containers_pauseable
			fi
			;;
	esac
}

_docker_port() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				__docker_containers_all
			fi
			;;
	esac
}

_docker_ps() {
	case "$prev" in
		--before|--since)
			__docker_containers_all
			;;
		--filter|-f)
			COMPREPLY=( $( compgen -S = -W "exited id label name status" -- "$cur" ) )
			compopt -o nospace
			return
			;;
		-n)
			return
			;;
	esac

	case "${words[$cword-2]}$prev=" in
		*id=*)
			cur="${cur#=}"
			__docker_container_ids
			return
			;;
		*name=*)
			cur="${cur#=}"
			__docker_container_names
			return
			;;
		*status=*)
			COMPREPLY=( $( compgen -W "exited paused restarting running" -- "${cur#=}" ) )
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--all -a --before --filter -f --help --latest -l -n --no-trunc --quiet -q --size -s --since" -- "$cur" ) )
			;;
	esac
}

_docker_pull() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--all-tags -a --help" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				for arg in "${COMP_WORDS[@]}"; do
					case "$arg" in
						--all-tags|-a)
							__docker_image_repos
							return
							;;
					esac
				done
				__docker_image_repos_and_tags
			fi
			;;
	esac
}

_docker_push() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				__docker_image_repos_and_tags
			fi
			;;
	esac
}

_docker_rename() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				__docker_containers_all
			fi
			;;
	esac
}

_docker_restart() {
	case "$prev" in
		--time|-t)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help --time -t" -- "$cur" ) )
			;;
		*)
			__docker_containers_all
			;;
	esac
}

_docker_rm() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--force -f --help --link -l --volumes -v" -- "$cur" ) )
			;;
		*)
			for arg in "${COMP_WORDS[@]}"; do
				case "$arg" in
					--force|-f)
						__docker_containers_all
						return
						;;
				esac
			done
			__docker_containers_stopped
			;;
	esac
}

_docker_rmi() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--force -f --help --no-prune" -- "$cur" ) )
			;;
		*)
			__docker_image_repos_and_tags_and_ids
			;;
	esac
}

_docker_run() {
	local options_with_args="
		--add-host
		--blkio-weight
		--attach -a
		--cap-add
		--cap-drop
		--cgroup-parent
		--cidfile
		--cpuset
		--cpu-period
		--cpu-quota
		--cpu-shares -c
		--device
		--dns
		--dns-search
		--entrypoint
		--env -e
		--env-file
		--expose
		--hostname -h
		--ipc
		--label -l
		--label-file
		--link
		--log-driver
		--lxc-conf
		--mac-address
		--memory -m
		--memory-swap
		--name
		--net
		--pid
		--publish -p
		--restart
		--security-opt
		--user -u
		--ulimit
		--uts
		--volumes-from
		--volume -v
		--workdir -w
	"

	local all_options="$options_with_args
		--help
		--interactive -i
		--privileged
		--publish-all -P
		--read-only
		--tty -t
	"

	[ "$command" = "run" ] && all_options="$all_options
		--detach -d
		--rm
		--sig-proxy
	"

	local options_with_args_glob=$(__docker_to_extglob "$options_with_args")

	case "$prev" in
		--add-host)
			case "$cur" in
				*:)
					__docker_resolve_hostname
					return
					;;
			esac
			;;
		--attach|-a)
			COMPREPLY=( $( compgen -W 'stdin stdout stderr' -- "$cur" ) )
			return
			;;
		--cap-add|--cap-drop)
			__docker_capabilities
			return
			;;
		--cidfile|--env-file|--label-file)
			_filedir
			return
			;;
		--device|--volume|-v)
			case "$cur" in
				*:*)
					# TODO somehow do _filedir for stuff inside the image, if it's already specified (which is also somewhat difficult to determine)
					;;
				'')
					COMPREPLY=( $( compgen -W '/' -- "$cur" ) )
					compopt -o nospace
					;;
				/*)
					_filedir
					compopt -o nospace
					;;
			esac
			return
			;;
		--env|-e)
			COMPREPLY=( $( compgen -e -- "$cur" ) )
			compopt -o nospace
			return
			;;
		--ipc)
			case "$cur" in
				*:*)
					cur="${cur#*:}"
					__docker_containers_running
					;;
				*)
					COMPREPLY=( $( compgen -W 'host container:' -- "$cur" ) )
					if [ "$COMPREPLY" = "container:" ]; then
						compopt -o nospace
					fi
					;;
			esac
			return
			;;
		--link)
			case "$cur" in
				*:*)
					;;
				*)
					__docker_containers_running
					COMPREPLY=( $( compgen -W "${COMPREPLY[*]}" -S ':' ) )
					compopt -o nospace
					;;
			esac
			return
			;;
		--log-driver)
			COMPREPLY=( $( compgen -W "json-file syslog none" -- "$cur") )
			return
			;;
		--net)
			case "$cur" in
				container:*)
					local cur=${cur#*:}
					__docker_containers_all
					;;
				*)
					COMPREPLY=( $( compgen -W "bridge none container: host" -- "$cur") )
					if [ "${COMPREPLY[*]}" = "container:" ] ; then
						compopt -o nospace
					fi
					;;
			esac
			return
			;;
		--restart)
			case "$cur" in
				on-failure:*)
					;;
				*)
					COMPREPLY=( $( compgen -W "no on-failure on-failure: always" -- "$cur") )
					;;
			esac
			return
			;;
		--security-opt)
			case "$cur" in
				label:*:*)
					;;
				label:*)
					local cur=${cur##*:}
					COMPREPLY=( $( compgen -W "user: role: type: level: disable" -- "$cur") )
					if [ "${COMPREPLY[*]}" != "disable" ] ; then
						compopt -o nospace
					fi
					;;
				*)
					COMPREPLY=( $( compgen -W "label apparmor" -S ":" -- "$cur") )
					compopt -o nospace
					;;
			esac
			return
			;;
		--volumes-from)
			__docker_containers_all
			return
			;;
		$options_with_args_glob )
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "$all_options" -- "$cur" ) )
			;;
		*)
			local counter=$( __docker_pos_first_nonflag $( __docker_to_alternatives "$options_with_args" ) )

			if [ $cword -eq $counter ]; then
				__docker_image_repos_and_tags_and_ids
			fi
			;;
	esac
}

_docker_save() {
	case "$prev" in
		--output|-o)
			_filedir
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help --output -o" -- "$cur" ) )
			;;
		*)
			__docker_image_repos_and_tags_and_ids
			;;
	esac
}

_docker_search() {
	case "$prev" in
		--stars|-s)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--automated --help --no-trunc --stars -s" -- "$cur" ) )
			;;
	esac
}

_docker_start() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--attach -a --help --interactive -i" -- "$cur" ) )
			;;
		*)
			__docker_containers_stopped
			;;
	esac
}

_docker_stats() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--no-stream --help" -- "$cur" ) )
			;;
		*)
			__docker_containers_running
			;;
	esac
}

_docker_stop() {
	case "$prev" in
		--time|-t)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help --time -t" -- "$cur" ) )
			;;
		*)
			__docker_containers_running
			;;
	esac
}

_docker_tag() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--force -f --help" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)

			if [ $cword -eq $counter ]; then
				__docker_image_repos_and_tags
				return
			fi
			(( counter++ ))

			if [ $cword -eq $counter ]; then
				__docker_image_repos_and_tags
				return
			fi
			;;
	esac
}

_docker_unpause() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				__docker_containers_unpauseable
			fi
			;;
	esac
}

_docker_top() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				__docker_containers_running
			fi
			;;
	esac
}

_docker_version() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
	esac
}

_docker_wait() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--help" -- "$cur" ) )
			;;
		*)
			__docker_containers_all
			;;
	esac
}

_docker() {
	local previous_extglob_setting=$(shopt -p extglob)
	shopt -s extglob

	local commands=(
		attach
		build
		commit
		cp
		create
		diff
		events
		exec
		export
		history
		images
		import
		info
		inspect
		kill
		load
		login
		logout
		logs
		pause
		port
		ps
		pull
		push
		rename
		restart
		rm
		rmi
		run
		save
		search
		start
		stats
		stop
		tag
		top
		unpause
		version
		wait
	)

	local main_options_with_args="
		--api-cors-header
		--bip
		--bridge -b
		--default-gateway
		--default-gateway-v6
		--default-ulimit
		--dns
		--dns-search
		--exec-driver -e
		--exec-opt
		--exec-root
		--fixed-cidr
		--fixed-cidr-v6
		--graph -g
		--group -G
		--host -H
		--insecure-registry
		--ip
		--label
		--log-driver
		--log-level -l
		--mtu
		--pidfile -p
		--registry-mirror
		--storage-driver -s
		--storage-opt
		--tlscacert
		--tlscert
		--tlskey
	"

	local main_options_with_args_glob=$(__docker_to_extglob "$main_options_with_args")
	local host

	COMPREPLY=()
	local cur prev words cword
	_get_comp_words_by_ref -n : cur prev words cword

	local command='docker' cpos=0
	local counter=1
	while [ $counter -lt $cword ]; do
		case "${words[$counter]}" in
			# save host so that completion can use custom daemon
			--host|-H)
				(( counter++ ))
				host="${words[$counter]}"
				;;
			$main_options_with_args_glob )
				(( counter++ ))
				;;
			-*)
				;;
			=)
				(( counter++ ))
				;;
			*)
				command="${words[$counter]}"
				cpos=$counter
				(( cpos++ ))
				break
				;;
		esac
		(( counter++ ))
	done

	local completions_func=_docker_${command}
	declare -F $completions_func >/dev/null && $completions_func

	eval "$previous_extglob_setting"
	return 0
}

complete -F _docker docker
