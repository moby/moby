#!bash

have docker && {
__docker_containers()
{
	local containers
	containers="$( docker ps -a -q )"
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
		__docker_containers
	fi
}

_docker_build()
{
	case "$prev" in
		-t)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-t -q" -- "$cur" ) )
			;;
		*)
			_filedir
			;;
	esac
}

_docker_commit()
{
	case "$prev" in
		-author|-m|-run)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-author -m -run" -- "$cur" ) )
			;;
		*)
			__docker_containers
			;;
	esac
}

_docker_diff()
{
	if [ $cpos -eq $cword ]; then
		__docker_containers
	fi
}

_docker_events()
{
	COMPREPLY=( $( compgen -W "-since" -- "$cur" ) )
}

_docker_export()
{
	if [ $cpos -eq $cword ]; then
		__docker_containers
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
			COMPREPLY=( $( compgen -W "-a -notrunc -q -viz" -- "$cur" ) )
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
	__docker_containers
}

_docker_login()
{
	COMPREPLY=( $( compgen -W "-e -p -u" -- "$cur" ) )
}

_docker_logs()
{
	if [ $cpos -eq $cword ]; then
		__docker_containers
	fi
}

_docker_port()
{
	if [ $cpos -eq $cword ]; then
		__docker_containers
	fi
}

_docker_ps()
{
	COMPREPLY=( $( compgen -W "-a -beforeId -l -n -notrunc -q -s -sinceId" -- "$cur" ) )
}

_docker_pull()
{
	COMPREPLY=( $( compgen -W "-t" -- "$cur" ) )
}

_docker_push()
{
	return
}

_docker_restart()
{
	case "$prev" in
		-t)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-t" -- "$cur" ) )
			;;
		*)
			__docker_containers
			;;
	esac
}

_docker_rm()
{
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-v" -- "$cur" ) )
			;;
		*)
			__docker_containers
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
		-cidfile)
			_filedir
			;;
		-volumes-from)
			__docker_containers
			;;
		-a|-c|-dns|-e|-h|-m|-p|-u|-v)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-a -c -cidfile -d -dns -e -entrypoint -h -i -m -n -p -t -u -v -volumes-from" -- "$cur" ) )
			;;
		*)
			case "$cur" in
				-*)
					COMPREPLY=( $( compgen -W "-a -notrunc -q -viz" -- "$cur" ) )
					;;
				*)
					local counter=$cpos
					while [ $counter -le $cword ]; do
						case "${words[$counter]}" in
							-a|-c|-cidfile|-dns|-e|-h|-m|-p|-u|-v|-volumes-from)
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
			;;
	esac
}

_docker_search()
{
	COMPREPLY=( $( compgen -W "-notrunc" -- "$cur" ) )
}

_docker_start()
{
	__docker_containers
}

_docker_stop()
{
	case "$prev" in
		-t)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-t" -- "$cur" ) )
			;;
		*)
			__docker_containers
			;;
	esac
}

_docker_tag()
{
	COMPREPLY=( $( compgen -W "-f" -- "$cur" ) )
}

_docker_top()
{
	if [ $cpos -eq $cword ]; then
		__docker_containers
	fi
}

_docker_version()
{
	return
}

_docker_wait()
{
	__docker_containers
}

_docker()
{
	local cur prev words cword command="docker" counter=1 word cpos
	local commands="
			attach
			build
			commit
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
}