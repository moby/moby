#!bash
#
# bash completion file for core docker commands
#
# This script provides supports completion of:
#  - commands and their options
#  - container ids and names
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

__docker_q() {
	docker 2>/dev/null "$@"
}

__docker_containers_all()
{
	local containers="$( __docker_q ps -a -q )"
	local names="$( __docker_q inspect --format '{{.Name}}' $containers | sed 's,^/,,' )"
	COMPREPLY=( $( compgen -W "$names $containers" -- "$cur" ) )
}

__docker_containers_running()
{
	local containers="$( __docker_q ps -q )"
	local names="$( __docker_q inspect --format '{{.Name}}' $containers | sed 's,^/,,' )"
	COMPREPLY=( $( compgen -W "$names $containers" -- "$cur" ) )
}

__docker_containers_stopped()
{
	local containers="$( { __docker_q ps -a -q; __docker_q ps -q; } | sort | uniq -u )"
	local names="$( __docker_q inspect --format '{{.Name}}' $containers | sed 's,^/,,' )"
	COMPREPLY=( $( compgen -W "$names $containers" -- "$cur" ) )
}

__docker_image_repos()
{
	local repos="$( __docker_q images | awk 'NR>1{print $1}' | grep -v '^<none>$' )"
	COMPREPLY=( $( compgen -W "$repos" -- "$cur" ) )
}

__docker_image_repos_and_tags()
{
	local repos="$( __docker_q images | awk 'NR>1{print $1}' | grep -v '^<none>$' )"
	local images="$( __docker_q images | awk 'NR>1{print $1":"$2}' | grep -v '^<none>:' )"
	COMPREPLY=( $( compgen -W "$repos $images" -- "$cur" ) )
	__ltrim_colon_completions "$cur"
}

__docker_image_repos_and_tags_and_ids()
{
	local repos="$( __docker_q images | awk 'NR>1{print $1}' | grep -v '^<none>$' )"
	local images="$( __docker_q images | awk 'NR>1{print $1":"$2}' | grep -v '^<none>:' )"
	local ids="$( __docker_q images -a -q )"
	COMPREPLY=( $( compgen -W "$repos $images $ids" -- "$cur" ) )
	__ltrim_colon_completions "$cur"
}

__docker_containers_and_images()
{
	local containers="$( __docker_q ps -a -q )"
	local names="$( __docker_q inspect --format '{{.Name}}' $containers | sed 's,^/,,' )"
	local repos="$( __docker_q images | awk 'NR>1{print $1}' | grep -v '^<none>$' )"
	local images="$( __docker_q images | awk 'NR>1{print $1":"$2}' | grep -v '^<none>:' )"
	local ids="$( __docker_q images -a -q )"
	COMPREPLY=( $( compgen -W "$containers $names $repos $images $ids" -- "$cur" ) )
	__ltrim_colon_completions "$cur"
}

__docker_pos_first_nonflag()
{
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

_docker_build()
{
	case "$prev" in
		-t|--tag)
			__docker_image_repos_and_tags
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-t --tag -q --quiet --no-cache --rm" -- "$cur" ) )
			;;
		*)
			local counter="$(__docker_pos_first_nonflag '-t|--tag')"
			if [ $cword -eq $counter ]; then
				_filedir
			fi
			;;
	esac
}

_docker_commit()
{
	case "$prev" in
		-m|--message|-a|--author|--run)
			return
			;;
		*)
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

_docker_cp()
{
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

_docker_diff()
{
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
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
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		__docker_containers_all
	fi
}

_docker_help()
{
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		COMPREPLY=( $( compgen -W "$commands" -- "$cur" ) )
	fi
}

_docker_history()
{
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

_docker_images()
{
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

_docker_import()
{
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

_docker_info()
{
	return
}

_docker_insert()
{
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		__docker_image_repos_and_tags_and_ids
	fi
}

_docker_inspect()
{
	case "$prev" in
		-f|--format)
			return
			;;
		*)
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

_docker_kill()
{
	__docker_containers_running
}

_docker_load()
{
	return
}

_docker_login()
{
	case "$prev" in
		-u|--username|-p|--password|-e|--email)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-u --username -p --password -e --email" -- "$cur" ) )
			;;
		*)
			;;
	esac
}

_docker_logs()
{
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

_docker_port()
{
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		__docker_containers_all
	fi
}

_docker_ps()
{
	case "$prev" in
		--since|--before)
 			__docker_containers_all
			;;
		-n)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-q --quiet -s --size -a --all --no-trunc -l --latest --since --before -n" -- "$cur" ) )
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
			local counter=$(__docker_pos_first_nonflag '-t|--tag')
			if [ $cword -eq $counter ]; then
				__docker_image_repos_and_tags
			fi
			;;
	esac
}

_docker_push()
{
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		__docker_image_repos
		# TODO replace this with __docker_image_repos_and_tags
		# see https://github.com/dotcloud/docker/issues/3411
	fi
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
			COMPREPLY=( $( compgen -W "-v --volumes -l --link" -- "$cur" ) )
			;;
		*)
			__docker_containers_stopped
			;;
	esac
}

_docker_rmi()
{
	__docker_image_repos_and_tags_and_ids
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
		-v|--volume)
			# TODO something magical with colons and _filedir ?
			return
			;;
		-e|--env)
			COMPREPLY=( $( compgen -e -- "$cur" ) )
			return
			;;
		--entrypoint|-h|--hostname|-m|--memory|-u|--user|-w|--workdir|-c|--cpu-shares|-n|--name|-a|--attach|--link|-p|--publish|--expose|--dns|--lxc-conf)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--rm -d --detach -n --networking --privileged -P --publish-all -i --interactive -t --tty --cidfile --entrypoint -h --hostname -m --memory -u --user -w --workdir -c --cpu-shares --sig-proxy --name -a --attach -v --volume --link -e --env -p --publish --expose --dns --volumes-from --lxc-conf" -- "$cur" ) )
			;;
		*)
			local counter=$(__docker_pos_first_nonflag '--cidfile|--volumes-from|-v|--volume|-e|--env|--entrypoint|-h|--hostname|-m|--memory|-u|--user|-w|--workdir|-c|--cpu-shares|-n|--name|-a|--attach|--link|-p|--publish|--expose|--dns|--lxc-conf')

			if [ $cword -eq $counter ]; then
				__docker_image_repos_and_tags_and_ids
			fi
			;;
	esac
}

_docker_save()
{
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
		__docker_image_repos_and_tags_and_ids
	fi
}

_docker_search()
{
	case "$prev" in
		-s|--stars)
			return
			;;
		*)
			;;
	esac

	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "--no-trunc --automated -s --stars" -- "$cur" ) )
			;;
		*)
			;;
	esac
}

_docker_start()
{
	case "$cur" in
		-*)
			COMPREPLY=( $( compgen -W "-a --attach -i --interactive" -- "$cur" ) )
			;;
		*)
			__docker_containers_stopped
			;;
	esac
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

_docker_top()
{
	local counter=$(__docker_pos_first_nonflag)
	if [ $cword -eq $counter ]; then
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
			load
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
			save
			search
			start
			stop
			tag
			top
			version
			wait
		"

	COMPREPLY=()
	local cur prev words cword
	_get_comp_words_by_ref -n : cur prev words cword

	local command='docker'
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
