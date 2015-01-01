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
	docker 2>/dev/null "$@"
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

_docker_docker() {
	case "$prev" in
		-H)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-H" -- "$cur" ) )
			;;
		*)
			COMPREPLY=( $( compgen -W "${commands[*]} help" -- "$cur" ) )
			;;
	esac
}

_docker_attach() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--no-stdin --sig-proxy" -- "$cur" ) )
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
		--tag|-t)
			__docker_image_repos_and_tags
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--force-rm --no-cache --quiet -q --rm --tag -t" -- "$cur" ) )
			;;
		*)
			local counter="$(__docker_pos_first_nonflag '--tag|-t')"
			if [ $cword -eq $counter ]; then
				_filedir -d
			fi
			;;
	esac
}

_docker_commit() {
	case "$prev" in
		--author|-a|--message|-m|--run)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--author -a --message -m --run" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag '--author|-a|--message|-m|--run')

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
		_filedir
		return
	fi
}

_docker_create() {
	_docker_run
}

_docker_diff() {
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		__docker_containers_all
	fi
}

_docker_events() {
	case "$prev" in
		--since)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--since" -- "$cur" ) )
			;;
	esac
}

_docker_exec() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--detach -d --interactive -i -t --tty" -- "$cur" ) )
			;;
		*)
			__docker_containers_running
			;;
	esac
}

_docker_export() {
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		__docker_containers_all
	fi
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
			COMPREPLY=( $( compgen -W "--no-trunc --quiet -q" -- "$cur" ) )
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
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--all -a --no-trunc --quiet -q" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				__docker_image_repos
			fi
			;;
	esac
}

_docker_import() {
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		return
	fi
	(( counter++ ))

	if [ $cword -eq $counter ]; then
		__docker_image_repos_and_tags
		return
	fi
}

_docker_info() {
	return
}

_docker_inspect() {
	case "$prev" in
		--format|-f)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--format -f" -- "$cur" ) )
			;;
		*)
			__docker_containers_and_images
			;;
	esac
}

_docker_kill() {
	__docker_containers_running
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
			COMPREPLY=( $( compgen -W "--input -i" -- "$cur" ) )
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
			COMPREPLY=( $( compgen -W "--email -e --password -p --username -u" -- "$cur" ) )
			;;
	esac
}

_docker_logs() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--follow -f" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag)
			if [ $cword -eq $counter ]; then
				__docker_containers_all
			fi
			;;
	esac
}

_docker_pause() {
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		__docker_containers_pauseable
	fi
}

_docker_port() {
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		__docker_containers_all
	fi
}

_docker_ps() {
	case "$prev" in
		--before|--since)
			__docker_containers_all
			;;
		-n)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--all -a --before --latest -l --no-trunc -n --quiet -q --size -s --since" -- "$cur" ) )
			;;
	esac
}

_docker_pull() {
	case "$prev" in
		--tag|-t)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--tag -t" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag '--tag|-t')
			if [ $cword -eq $counter ]; then
				__docker_image_repos_and_tags
			fi
			;;
	esac
}

_docker_push() {
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		__docker_image_repos_and_tags
	fi
}

_docker_restart() {
	case "$prev" in
		--time|-t)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--time -t" -- "$cur" ) )
			;;
		*)
			__docker_containers_all
			;;
	esac
}

_docker_rm() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--force -f --link -l --volumes -v" -- "$cur" ) )
			return
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
			return
			;;
	esac
}

_docker_rmi() {
	__docker_image_repos_and_tags_and_ids
}

_docker_run() {
	local options_with_args="
		--add-host
		--attach -a
		--cap-add
		--cap-drop
		--cidfile
		--cpuset
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
		--link
		--lxc-conf
		--mac-address
		--memory -m
		--name
		--net
		--publish -p
		--restart
		--security-opt
		--user -u
		--volumes-from
		--volume -v
		--workdir -w
	"

	local all_options="$options_with_args
		--interactive -i
		--privileged
		--publish-all -P
		--tty -t
	"

	[ "$command" = "run" ] && all_options="$all_options
		--detach -d
		--rm
		--sig-proxy
	"

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
		--cidfile|--env-file)
			_filedir
			return
			;;
		--device|-d|--volume)
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
		--cpuset|--cpu-shares|-c|--dns|--dns-search|--entrypoint|--expose|--hostname|-h|--lxc-conf|--mac-address|--memory|-m|--name|-n|--publish|-p|--user|-u|--workdir|-w)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "$all_options" -- "$cur" ) )
			;;
		*)
			local counter=$( __docker_pos_first_nonflag $( echo $options_with_args | tr -d "\n" | tr " " "|" ) )

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
			COMPREPLY=( $( compgen -W "-o --output" -- "$cur" ) )
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
			COMPREPLY=( $( compgen -W "--automated --no-trunc --stars -s" -- "$cur" ) )
			;;
	esac
}

_docker_start() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--attach -a --interactive -i" -- "$cur" ) )
			;;
		*)
			__docker_containers_stopped
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
			COMPREPLY=( $( compgen -W "--time -t" -- "$cur" ) )
			;;
		*)
			__docker_containers_running
			;;
	esac
}

_docker_tag() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--force -f" -- "$cur" ) )
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
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		__docker_containers_unpauseable
	fi
}

_docker_top() {
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		__docker_containers_running
	fi
}

_docker_version() {
	return
}

_docker_wait() {
	__docker_containers_all
}

_docker() {
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
		insert
		inspect
		kill
		load
		login
		logs
		pause
		port
		ps
		pull
		push
		restart
		rm
		rmi
		run
		save
		search
		start
		stop
		tag
		top
		unpause
		version
		wait
	)

	COMPREPLY=()
	local cur prev words cword
	_get_comp_words_by_ref -n : cur prev words cword

	local command='docker' cpos=0
	local counter=1
	while [ $counter -lt $cword ]; do
		case "${words[$counter]}" in
			-H)
				(( counter++ ))
				;;
			-*)
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

	return 0
}

complete -F _docker docker
