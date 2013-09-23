#!bash
#
# bash completion file for core docker commands
#
# This script provides supports completion of:
#  - commands and their options
#  - container ids
#  - image repos and tags
#  - filepaths
#
# To enable the completions either:
#  - place this file in /etc/bash_completion.d
#  or
#  - copy this file and add the line below to your .bashrc after
#    bash completion features are loaded
#     . docker.bash
#
# Note:
# Currently, the completions will not work if the docker daemon is not
# bound to the default communication port/socket
# If the docker daemon is using a unix socket for communication your user
# must have access to the socket for the completions to function correctly

__docker_containers_all()
{
	local containers
	containers="$( docker ps -a -q )"
	COMPREPLY=( $( compgen -W "$containers" -- "$cur" ) )
}

__docker_containers_running()
{
	local containers
	containers="$( docker ps -q )"
	COMPREPLY=( $( compgen -W "$containers" -- "$cur" ) )
}

__docker_containers_stopped()
{
	local containers
	containers="$( comm -13 <(docker ps -q | sort -u) <(docker ps -a -q | sort -u) )"
	COMPREPLY=( $( compgen -W "$containers" -- "$cur" ) )
}

__docker_image_repos()
{
	local repos
	repos="$( docker images | awk 'NR>1{print $1}' )"
	COMPREPLY=( $( compgen -W "$repos" -- "$cur" ) )
}

__docker_images()
{
	local images
	images="$( docker images | awk 'NR>1{print $1":"$2}' )"
	COMPREPLY=( $( compgen -W "$images" -- "$cur" ) )
	__ltrim_colon_completions "$cur"
}

__docker_image_repos_and_tags()
{
	local repos images
	repos="$( docker images | awk 'NR>1{print $1}' )"
	images="$( docker images | awk 'NR>1{print $1":"$2}' )"
	COMPREPLY=( $( compgen -W "$repos $images" -- "$cur" ) )
	__ltrim_colon_completions "$cur"
}

__docker_containers_and_images()
{
	local containers images
	containers="$( docker ps -a -q )"
	images="$( docker images | awk 'NR>1{print $1":"$2}' )"
	COMPREPLY=( $( compgen -W "$images $containers" -- "$cur" ) )
	__ltrim_colon_completions "$cur"
}

_docker_docker()
{
	case "$prev" in
		-H)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-H" -- "$cur" ) )
			;;
		*)
			COMPREPLY=( $( compgen -W "$commands help" -- "$cur" ) )
			;;
	esac
}

_docker_attach()
{
	if [ $cpos -eq $cword ]; then
		__docker_containers_running
	fi
}

_docker_build()
{
	case "$prev" in
		-t|--tag)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--no-cache -t --tag -q --quiet --rm" -- "$cur" ) )
			;;
		*)
			_filedir
			;;
	esac
}

_docker_commit()
{
	case "$prev" in
		-a|--author|-m|--message|-r|--run)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-a --author -m --message -r --run" -- "$cur" ) )
			;;
		*)
			local counter=$cpos
			while [ $counter -le $cword ]; do
				case "${words[$counter]}" in
					-a|--author|-m|--message|-r|--run)
						(( counter++ ))
						;;
					-*)
						;;
					*)
						break
						;;
				esac
				(( counter++ ))
			done

			if [ $counter -eq $cword ]; then
				__docker_containers_all
			fi
			;;
	esac
}

_docker_cp()
{
	if [ $cpos -eq $cword ]; then
		__docker_containers_all
	else
		_filedir
	fi
}

_docker_diff()
{
	if [ $cpos -eq $cword ]; then
		__docker_containers_all
	fi
}

_docker_events()
{
	case "$prev" in
		--since)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--since" -- "$cur" ) )
			;;
		*)
			;;
	esac
}

_docker_export()
{
	if [ $cpos -eq $cword ]; then
		__docker_containers_all
	fi
}

_docker_help()
{
	if [ $cpos -eq $cword ]; then
		COMPREPLY=( $( compgen -W "$commands" -- "$cur" ) )
	fi
}

_docker_history()
{
	if [ $cpos -eq $cword ]; then
		__docker_image_repos_and_tags
	fi
}

_docker_images()
{
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-a --all --no-trunc -q --quiet --viz" -- "$cur" ) )
			;;
		*)
			local counter=$cpos
			while [ $counter -le $cword ]; do
				case "${words[$counter]}" in
					-*)
						;;
					*)
						break
						;;
				esac
				(( counter++ ))
			done

			if [ $counter -eq $cword ]; then
				__docker_image_repos
			fi
			;;
	esac
}

_docker_import()
{
	return
}

_docker_info()
{
	return
}

_docker_insert()
{
	if [ $cpos -eq $cword ]; then
		__docker_image_repos_and_tags
	fi
}

_docker_inspect()
{
	__docker_containers_and_images
}

_docker_kill()
{
	__docker_containers_running
}

_docker_login()
{
	case "$prev" in
		-e|--email|-p|--password|-u|--username)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-e --email -p --password -u --username" -- "$cur" ) )
			;;
		*)
			;;
	esac
}

_docker_logs()
{
	if [ $cpos -eq $cword ]; then
		__docker_containers_all
	fi
}

_docker_port()
{
	if [ $cpos -eq $cword ]; then
		__docker_containers_all
	fi
}

_docker_ps()
{
	case "$prev" in
		--before|-n|--since)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-a --all --before -l --latest -n --no-trunc -q --quiet -s --size --since" -- "$cur" ) )
			;;
		*)
			;;
	esac
}

_docker_pull()
{
	case "$prev" in
		-t|--tag)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-t --tag" -- "$cur" ) )
			;;
		*)
			;;
	esac
}

_docker_push()
{
	return
}

_docker_restart()
{
	case "$prev" in
		-t|--time)
			return
			;;
		*)
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

_docker_rm()
{
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-v --volumes" -- "$cur" ) )
			;;
		*)
			__docker_containers_stopped
			;;
	esac
}

_docker_rmi()
{
	__docker_image_repos_and_tags
}

_docker_run()
{
	case "$prev" in
		--cidfile)
			_filedir
			;;
		--volumes-from)
			__docker_containers_all
			;;
		-a|--attach|-c|--cpu|--dns|-e|--env|--entrypoint|-h|--host|--lxc-conf|-m|--memory|-p|--port|-u|--username|-v|--volume|-w|--workdir)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-a --attach -c --cpu --cidfile -d --detached --dns -e --env --entrypoint -h --host -i --interactive --lxc-conf -m --memory -n --networking -p --port --privileged -t --tty -u --user -v --volume --volumes-from -w --workdir" -- "$cur" ) )
			;;
		*)
			local counter=$cpos
			while [ $counter -le $cword ]; do
				case "${words[$counter]}" in
					-a|--attach|-c|--cpu|--cidfile|--dns|-e|--env|--entrypoint|-h|--host|--lxc-conf|-m|--message|-p|--port|-u|--user|-v|--volume|--volumes-from|-w|--workdir)
						(( counter++ ))
						;;
					-*)
						;;
					*)
						break
						;;
				esac
				(( counter++ ))
			done

			if [ $counter -eq $cword ]; then
				__docker_image_repos_and_tags
			fi
			;;
	esac
}

_docker_search()
{
	COMPREPLY=( $( compgen -W "--no-trunc" -- "$cur" ) )
}

_docker_start()
{
	__docker_containers_stopped
}

_docker_stop()
{
	case "$prev" in
		-t|--time)
			return
			;;
		*)
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

_docker_tag()
{
	COMPREPLY=( $( compgen -W "-f --force" -- "$cur" ) )
}

_docker_top()
{
	if [ $cpos -eq $cword ]; then
		__docker_containers_running
	fi
}

_docker_version()
{
	return
}

_docker_wait()
{
	__docker_containers_all
}

_docker()
{
	local cur prev words cword command="docker" counter=1 word cpos
	local commands="
			attach
			build
			commit
			cp
			diff
			events
			export
			history
			images
			import
			info
			insert
			inspect
			kill
			login
			logs
			port
			ps
			pull
			push
			restart
			rm
			rmi
			run
			search
			start
			stop
			tag
			top
			version
			wait
		"

	COMPREPLY=()
	_get_comp_words_by_ref -n : cur prev words cword

	while [ $counter -lt $cword ]; do
		word="${words[$counter]}"
		case "$word" in
			-H)
				(( counter++ ))
				;;
			-*)
				;;
			*)
				command="$word"
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
