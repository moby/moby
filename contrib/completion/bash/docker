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
		-t|--tag)
			__docker_image_repos_and_tags
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-t --tag -q --quiet --no-cache --rm --force-rm" -- "$cur" ) )
			;;
		*)
			local counter="$(__docker_pos_first_nonflag '-t|--tag')"
			if [ $cword -eq $counter ]; then
				_filedir -d
			fi
			;;
	esac
}

_docker_commit() {
	case "$prev" in
		-m|--message|-a|--author|--run)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-m --message -a --author --run" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag '-m|--message|-a|--author|--run')

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
	case "$prev" in
		-a|--attach)
			COMPREPLY=( $( compgen -W 'stdin stdout stderr' -- "$cur" ) )
			return
			;;
		--cidfile|--env-file)
			_filedir
			return
			;;
		--volumes-from)
			__docker_containers_all
			return
			;;
		-v|--volume|--device)
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
		-e|--env)
			COMPREPLY=( $( compgen -e -- "$cur" ) )
			compopt -o nospace
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
		--add-host)
			case "$cur" in
				*:)
					__docker_resolve_hostname
					return
					;;
			esac
			;;
		--cap-add|--cap-drop)
			__docker_capabilities
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
		--entrypoint|-h|--hostname|-m|--memory|-u|--user|-w|--workdir|--cpuset|-c|--cpu-shares|-n|--name|-p|--publish|--expose|--dns|--lxc-conf|--dns-search)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--privileged -P --publish-all -i --interactive -t --tty --cidfile --entrypoint -h --hostname -m --memory -u --user -w --workdir --cpuset -c --cpu-shares --name -a --attach -v --volume --link -e --env --env-file -p --publish --expose --dns --volumes-from --lxc-conf --security-opt --add-host --cap-add --cap-drop --device --dns-search --net --restart" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag '--cidfile|--volumes-from|-v|--volume|-e|--env|--env-file|--entrypoint|-h|--hostname|-m|--memory|-u|--user|-w|--workdir|--cpuset|-c|--cpu-shares|-n|--name|-a|--attach|--link|-p|--publish|--expose|--dns|--lxc-conf|--security-opt|--add-host|--cap-add|--cap-drop|--device|--dns-search|--net|--restart')

			if [ $cword -eq $counter ]; then
				__docker_image_repos_and_tags_and_ids
			fi
			;;
	esac
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
			COMPREPLY=( $( compgen -W "-d --detach -i --interactive -t --tty" -- "$cur" ) )
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
			COMPREPLY=( $( compgen -W "-q --quiet --no-trunc" -- "$cur" ) )
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
			COMPREPLY=( $( compgen -W "-q --quiet -a --all --no-trunc -v --viz -t --tree" -- "$cur" ) )
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
		-f|--format)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-f --format" -- "$cur" ) )
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
	return
}

_docker_login() {
	case "$prev" in
		-u|--username|-p|--password|-e|--email)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-u --username -p --password -e --email" -- "$cur" ) )
			;;
	esac
}

_docker_logs() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-f --follow" -- "$cur" ) )
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
		--since|--before)
			__docker_containers_all
			;;
		-n)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-q --quiet -s --size -a --all --no-trunc -l --latest --since --before -n" -- "$cur" ) )
			;;
	esac
}

_docker_pull() {
	case "$prev" in
		-t|--tag)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-t --tag" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag '-t|--tag')
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
		-t|--time)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-t --time" -- "$cur" ) )
			;;
		*)
			__docker_containers_all
			;;
	esac
}

_docker_rm() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-f --force -l --link -v --volumes" -- "$cur" ) )
			return
			;;
		*)
			for arg in "${COMP_WORDS[@]}"; do
				case "$arg" in
					-f|--force)
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
	case "$prev" in
		-a|--attach)
			COMPREPLY=( $( compgen -W 'stdin stdout stderr' -- "$cur" ) )
			return
			;;
		--cidfile|--env-file)
			_filedir
			return
			;;
		--volumes-from)
			__docker_containers_all
			return
			;;
		-v|--volume|--device)
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
		-e|--env)
			COMPREPLY=( $( compgen -e -- "$cur" ) )
			compopt -o nospace
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
		--add-host)
			case "$cur" in
				*:)
					__docker_resolve_hostname
					return
					;;
			esac
			;;
		--cap-add|--cap-drop)
			__docker_capabilities
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
		--entrypoint|-h|--hostname|-m|--memory|-u|--user|-w|--workdir|--cpuset|-c|--cpu-shares|-n|--name|-p|--publish|--expose|--dns|--lxc-conf|--dns-search)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--rm -d --detach --privileged -P --publish-all -i --interactive -t --tty --cidfile --entrypoint -h --hostname -m --memory -u --user -w --workdir --cpuset -c --cpu-shares --sig-proxy --name -a --attach -v --volume --link -e --env --env-file -p --publish --expose --dns --volumes-from --lxc-conf --security-opt --add-host --cap-add --cap-drop --device --dns-search --net --restart" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag '--cidfile|--volumes-from|-v|--volume|-e|--env|--env-file|--entrypoint|-h|--hostname|-m|--memory|-u|--user|-w|--workdir|--cpuset|-c|--cpu-shares|-n|--name|-a|--attach|--link|-p|--publish|--expose|--dns|--lxc-conf|--security-opt|--add-host|--cap-add|--cap-drop|--device|--dns-search|--net|--restart')

			if [ $cword -eq $counter ]; then
				__docker_image_repos_and_tags_and_ids
			fi
			;;
	esac
}

_docker_save() {
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		__docker_image_repos_and_tags_and_ids
	fi
}

_docker_search() {
	case "$prev" in
		-s|--stars)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--no-trunc --automated -s --stars" -- "$cur" ) )
			;;
	esac
}

_docker_start() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-a --attach -i --interactive" -- "$cur" ) )
			;;
		*)
			__docker_containers_stopped
			;;
	esac
}

_docker_stop() {
	case "$prev" in
		-t|--time)
			return
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-t --time" -- "$cur" ) )
			;;
		*)
			__docker_containers_running
			;;
	esac
}

_docker_tag() {
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-f --force" -- "$cur" ) )
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
