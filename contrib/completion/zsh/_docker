#compdef docker
#
# zsh completion for docker (http://docker.com)
#
# version:  0.3.0
# github:   https://github.com/felixr/docker-zsh-completion
#
# contributers:
#   - Felix Riedel
#   - Vincent Bernat
#
# license:
#
# Copyright (c) 2013, Felix Riedel
# All rights reserved.
#
# Redistribution and use in source and binary forms, with or without
# modification, are permitted provided that the following conditions are met:
#     * Redistributions of source code must retain the above copyright
#       notice, this list of conditions and the following disclaimer.
#     * Redistributions in binary form must reproduce the above copyright
#       notice, this list of conditions and the following disclaimer in the
#       documentation and/or other materials provided with the distribution.
#     * Neither the name of the <organization> nor the
#       names of its contributors may be used to endorse or promote products
#       derived from this software without specific prior written permission.
#
# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
# ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
# WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
# DISCLAIMED. IN NO EVENT SHALL <COPYRIGHT HOLDER> BE LIABLE FOR ANY
# DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
# (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
# LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
# ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
# (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
# SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
#

__parse_docker_list() {
        awk '
NR == 1 {
    idx=1;i=0;f[i]=0
    header=$0
    while ( match(header, /  ([A-Z]+|[A-Z]+ [A-Z]+)/) ) {
        idx += RSTART+1
        f[++i]=idx
        header = substr($0,idx)
    }
    f[++i]=999
}

NR > 1 '"$1"' {
    for(j=0;j<i;j++) {
        x[j] = substr($0, f[j], f[j+1]-f[j]-1)
        gsub(/[ ]+$/, "", x[j])
    }
    printf("%s:%7s, %s\n", x[0], x[3], x[1])
    if (x[6] != "") {
       split(x[6], names, /,/)
       for (name in names) printf("%s:%7s, %s\n", names[name], x[3], x[1])
    }
}
'| sed -e 's/ \([hdwm]\)\(inutes\|ays\|ours\|eeks\)/\1/'
}

__docker_stoppedcontainers() {
    local expl
    declare -a stoppedcontainers
    stoppedcontainers=(${(f)"$(_call_program commands docker ps -a |  __parse_docker_list '&& / Exit/')"})
    _describe -t containers-stopped "Stopped Containers" stoppedcontainers "$@"
}

__docker_runningcontainers() {
    local expl
    declare -a containers

    containers=(${(f)"$(_call_program commands docker ps | __parse_docker_list)"})
    _describe -t containers-active "Running Containers" containers "$@"
}

__docker_containers () {
    __docker_stoppedcontainers "$@"
    __docker_runningcontainers "$@"
}

__docker_images () {
    local expl
    declare -a images
    images=(${(f)"$(_call_program commands docker images | awk '(NR > 1 && $1 != "<none>"){printf("%s", $1);if ($2 != "<none>") printf("\\:%s", $2); printf("\n")}')"})
    images=($images ${(f)"$(_call_program commands docker images | awk '(NR > 1){printf("%s:%-15s in %s\n", $3,$2,$1)}')"})
    _describe -t docker-images "Images" images
}

__docker_tags() {
    local expl
    declare -a tags
    tags=(${(f)"$(_call_program commands docker images | awk '(NR>1){print $2}'| sort | uniq)"})
    _describe -t docker-tags "tags" tags
}

__docker_repositories_with_tags() {
    if compset -P '*:'; then
        __docker_tags
    else
        __docker_repositories -qS ":"
    fi
}

__docker_search() {
    # declare -a dockersearch
    local cache_policy
    zstyle -s ":completion:${curcontext}:" cache-policy cache_policy
    if [[ -z "$cache_policy" ]]; then
        zstyle ":completion:${curcontext}:" cache-policy __docker_caching_policy
    fi

    local searchterm cachename
    searchterm="${words[$CURRENT]%/}"
    cachename=_docker-search-$searchterm

    local expl
    local -a result
    if ( [[ ${(P)+cachename} -eq 0 ]] || _cache_invalid ${cachename#_} ) \
        && ! _retrieve_cache ${cachename#_}; then
        _message "Searching for ${searchterm}..."
        result=(${(f)"$(_call_program commands docker search ${searchterm} | awk '(NR>2){print $1}')"})
        _store_cache ${cachename#_} result
    fi
    _wanted dockersearch expl 'Available images' compadd -a result
}

__docker_caching_policy()
{
  # oldp=( "$1"(Nmh+24) )     # 24 hour
  oldp=( "$1"(Nmh+1) )     # 24 hour
  (( $#oldp ))
}


__docker_repositories () {
    local expl
    declare -a repos
    repos=(${(f)"$(_call_program commands docker images | sed -e '1d' -e 's/[ ].*//' | sort | uniq)"})
    _describe -t docker-repos "Repositories" repos "$@"
}

__docker_commands () {
    # local -a  _docker_subcommands
    local cache_policy

    zstyle -s ":completion:${curcontext}:" cache-policy cache_policy
    if [[ -z "$cache_policy" ]]; then
        zstyle ":completion:${curcontext}:" cache-policy __docker_caching_policy
    fi

    if ( [[ ${+_docker_subcommands} -eq 0 ]] || _cache_invalid docker_subcommands) \
        && ! _retrieve_cache docker_subcommands;
    then
        _docker_subcommands=(${${(f)"$(_call_program commands
        docker 2>&1 | sed -e '1,6d' -e '/^[ ]*$/d' -e 's/[ ]*\([^ ]\+\)\s*\([^ ].*\)/\1:\2/' )"}})
        _docker_subcommands=($_docker_subcommands 'help:Show help for a command')
        _store_cache docker_subcommands _docker_subcommands
    fi
    _describe -t docker-commands "docker command" _docker_subcommands
}

__docker_subcommand () {
    local -a _command_args
    case "$words[1]" in
        (attach)
            _arguments \
                '--no-stdin[Do not attach stdin]' \
                '--sig-proxy[Proxify all received signal]' \
                ':containers:__docker_runningcontainers'
            ;;
        (build)
            _arguments \
                '--no-cache[Do not use cache when building the image]' \
                '-q[Suppress verbose build output]' \
                '--rm[Remove intermediate containers after a successful build]' \
                '-t=-:repository:__docker_repositories_with_tags' \
                ':path or URL:_directories'
            ;;
        (commit)
            _arguments \
                '--author=-[Author]:author: ' \
                '-m=-[Commit message]:message: ' \
                '--run=-[Configuration automatically applied when the image is run]:configuration: ' \
                ':container:__docker_containers' \
                ':repository:__docker_repositories_with_tags'
            ;;
        (cp)
            _arguments \
                ':container:->container' \
                ':hostpath:_files'
            case $state in
                (container)
                    if compset -P '*:'; then
                        _files
                    else
                        __docker_containers -qS ":"
                    fi
                    ;;
            esac
            ;;
        (diff|export)
            _arguments '*:containers:__docker_containers'
            ;;
        (history)
            _arguments \
                '--no-trunc[Do not truncate output]' \
                '-q[Only show numeric IDs]' \
                '*:images:__docker_images'
            ;;
        (images)
            _arguments \
                '-a[Show all images]' \
                '--no-trunc[Do not truncate output]' \
                '-q[Only show numeric IDs]' \
                '--tree[Output graph in tree format]' \
                '--viz[Output graph in graphviz format]' \
                ':repository:__docker_repositories'
            ;;
        (inspect)
            _arguments \
                '--format=-[Format the output using the given go template]:template: ' \
                '*:containers:__docker_containers'
            ;;
        (import)
            _arguments \
                ':URL:(- http:// file://)' \
                ':repository:__docker_repositories_with_tags'
            ;;
        (info)
            ;;
        (import)
            _arguments \
                ':URL:(- http:// file://)' \
                ':repository:__docker_repositories_with_tags'
            ;;
        (insert)
            _arguments '1:containers:__docker_containers' \
                       '2:URL:(http:// file://)' \
                       '3:file:_files'
            ;;
        (kill)
            _arguments '*:containers:__docker_runningcontainers'
            ;;
        (load)
            ;;
        (login)
            _arguments \
                '-e=-[Email]:email: ' \
                '-p=-[Password]:password: ' \
                '-u=-[Username]:username: ' \
                ':server: '
            ;;
        (logs)
            _arguments \
                '-f[Follow log output]' \
                '*:containers:__docker_containers'
            ;;
        (port)
            _arguments \
                '1:containers:__docker_runningcontainers' \
                '2:port:_ports'
            ;;
        (start)
            _arguments \
                '-a[Attach container'"'"'s stdout/stderr and forward all signals]' \
                '-i[Attach container'"'"'s stding]' \
                '*:containers:__docker_stoppedcontainers'
            ;;
        (rm)
            _arguments \
                '--link[Remove the specified link and not the underlying container]' \
                '-v[Remove the volumes associated to the container]' \
                '*:containers:__docker_stoppedcontainers'
            ;;
        (rmi)
            _arguments \
                '*:images:__docker_images'
            ;;
        (restart|stop)
            _arguments '-t=-[Number of seconds to try to stop for before killing the container]:seconds to before killing:(1 5 10 30 60)' \
                '*:containers:__docker_runningcontainers'
            ;;
        (top)
            _arguments \
                '1:containers:__docker_runningcontainers' \
                '(-)*:: :->ps-arguments'
            case $state in
                (ps-arguments)
                    _ps
                    ;;
            esac

            ;;
        (ps)
            _arguments \
                '-a[Show all containers]' \
                '--before=-[Show only container created before...]:containers:__docker_containers' \
                '-l[Show only the latest created container]' \
                '-n=-[Show n last created containers, include non-running one]:n:(1 5 10 25 50)' \
                '--no-trunc[Do not truncate output]' \
                '-q[Only show numeric IDs]' \
                '-s[Display sizes]' \
                '--since=-[Show only containers created since...]:containers:__docker_containers'
            ;;
        (tag)
            _arguments \
                '-f[force]'\
                ':image:__docker_images'\
                ':repository:__docker_repositories_with_tags'
            ;;
        (run)
            _arguments \
                '-P[Publish all exposed ports to the host]' \
                '-a[Attach to stdin, stdout or stderr]' \
                '-c=-[CPU shares (relative weight)]:CPU shares:(0 10 100 200 500 800 1000)' \
                '--cidfile=-[Write the container ID to the file]:CID file:_files' \
                '-d[Detached mode: leave the container running in the background]' \
                '*--dns=-[Set custom dns servers]:dns server: ' \
                '*-e=-[Set environment variables]:environment variable: ' \
                '--entrypoint=-[Overwrite the default entrypoint of the image]:entry point: ' \
                '*--expose=-[Expose a port from the container without publishing it]: ' \
                '-h=-[Container host name]:hostname:_hosts' \
                '-i[Keep stdin open even if not attached]' \
                '--link=-[Add link to another container]:link:->link' \
                '--lxc-conf=-[Add custom lxc options]:lxc options: ' \
                '-m=-[Memory limit (in bytes)]:limit: ' \
                '--name=-[Container name]:name: ' \
                '*-p=-[Expose a container'"'"'s port to the host]:port:_ports' \
                '--privileged[Give extended privileges to this container]' \
                '--rm[Remove intermediate containers when it exits]' \
                '--sig-proxy[Proxify all received signal]' \
                '-t[Allocate a pseudo-tty]' \
                '-u=-[Username or UID]:user:_users' \
                '*-v=-[Bind mount a volume (e.g. from the host: -v /host:/container, from docker: -v /container)]:volume: '\
                '--volumes-from=-[Mount volumes from the specified container]:volume: ' \
                '-w=-[Working directory inside the container]:directory:_directories' \
                '(-):images:__docker_images' \
                '(-):command: _command_names -e' \
                '*::arguments: _normal'

            case $state in
                (link)
                    if compset -P '*:'; then
                        _wanted alias expl 'Alias' compadd -E ""
                    else
                        __docker_runningcontainers -qS ":"
                    fi
                    ;;
            esac

            ;;
        (pull|search)
            _arguments ':name:__docker_search'
            ;;
        (push)
            _arguments ':repository:__docker_repositories_with_tags'
            ;;
        (save)
            _arguments \
                ':images:__docker_images'
            ;;
        (wait)
            _arguments ':containers:__docker_runningcontainers'
            ;;
        (help)
            _arguments ':subcommand:__docker_commands'
            ;;
        (*)
            _message 'Unknown sub command'
    esac

}

_docker () {
    # Support for subservices, which allows for `compdef _docker docker-shell=_docker_containers`.
    # Based on /usr/share/zsh/functions/Completion/Unix/_git without support for `ret`.
    if [[ $service != docker ]]; then
        _call_function - _$service
        return
    fi

    local curcontext="$curcontext" state line
    typeset -A opt_args

    _arguments -C \
      '-H=-[tcp://host:port to bind/connect to]:socket: ' \
         '(-): :->command' \
         '(-)*:: :->option-or-argument'

    if (( CURRENT == 1 )); then

    fi
    case $state in
        (command)
            __docker_commands
            ;;
        (option-or-argument)
            curcontext=${curcontext%:*:*}:docker-$words[1]:
            __docker_subcommand
            ;;
    esac
}

_docker "$@"
