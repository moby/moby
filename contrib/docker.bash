_docker_procs()
{
  local procs
  procs="$(docker ps -q)"
  COMPREPLY=( $(compgen -W "${procs}" -- ${cur}) )
}

_docker_imgs()
{
  local repos tags imgs
  repos="$(docker images|awk 'NR>1{print $1}'|sort -u)"
  tags="$(docker images|awk 'NR>1{print $1":"$2}'|sort -u)"
  imgs="$(docker images -a -q|sort -u) $repos $tags"
  COMPREPLY=( $(compgen -W "$imgs" -- $cur) )
  _docker_fix_colon_reply
}

_docker_fix_colon_reply()
{
  # work around completion issues with : separator
  # eg. base:base:latest vs. base:latest
  local colprefs i
  colprefs=${cur%"${cur##*:}"}
  i=${#COMPREPLY[*]}
  while [ $((--i)) -ge 0 ]; do
    COMPREPLY[$i]=${COMPREPLY[$i]#"$colprefs"} 
  done
}

_docker() 
{
  local cur prev cmds curcmd i repos
  COMPREPLY=()
  #cur="${COMP_WORDS[COMP_CWORD]}"
  #prev="${COMP_WORDS[COMP_CWORD-1]}"
  _get_comp_words_by_ref -n : cur prev
  cmds="attach|build|commit|diff|export|history|images|import|info|insert|inspect|kill|login|logs|port|ps|pull|push|restart|rm|rmi|run|search|start|stop|tag|version|wait"

  for (( i=0; i < ${#COMP_WORDS[@]}-1; i++ )); do
    if [[ ${COMP_WORDS[i]} == @($cmds) ]]; then
      curcmd=${COMP_WORDS[i]}
    fi
  done

  #echo "cur='$cur' prev='$prev'"
  case $curcmd in
    attach|inspect|diff|export|logs|port)
      _docker_procs
      return 0
      ;;
    kill|restart|start|wait)
      # supports multiple containers
      _docker_procs
      return 0
      ;;
    history|insert)
      _docker_imgs
      return 0
      ;;
    rmi)
      # support multiple images
      _docker_imgs
      return 0
      ;;
    commit)
      if [[ $cur == -* ]]; then
        COMPREPLY=( $(compgen -W "-author -m -run" -- $cur) )
      else
        _docker_procs
      fi
      return 0
      ;;
    images)
      if [[ $cur == -* ]]; then
        COMPREPLY=( $(compgen -W "-a -q -viz" -- $cur) )
      else
       repos="$(docker images|awk 'NR>1{print $1}'|sort -u)"
       COMPREPLY=( $(compgen -W "$repos" -- $cur) )
      fi
      return 0
      ;;
    import)
      _filedir
      return 0
      ;;
    ps)
      COMPREPLY=( $(compgen -W "-a -l -n -notrunc -q" -- $cur) )
      return 0
      ;;
    pull)
      if [[ $cur == -* ]]; then
        COMPREPLY=( $(compgen -W "-registry -t" -- $cur) )
      elif [[ -n "$cur" ]]; then
       repos="$(docker search $cur|awk 'NR>2{print $1}'|sort -u)"
       COMPREPLY=( $(compgen -W "$repos" -- $cur) )
      fi
      return 0
      ;;
    push)
      COMPREPLY=( $(compgen -W "-registry" -- $cur) )
      return 0
      ;;
    rm)
      # supports multiple containers
      if [[ $cur == -* ]]; then
        COMPREPLY=( $(compgen -W "-v" -- $cur) )
      else
        _docker_procs
      fi
      return 0
      ;;
    run)
      if [[ $cur == -* ]]; then
        COMPREPLY=( $(compgen -W "-a -c -d -e -h -i -m -p -t -u -d -v -volumes-from" -- $cur) )
      else
        _docker_imgs
      fi
      return 0
      ;;
    stop)
      # supports multiple containers
      if [[ $cur == -* ]]; then
        COMPREPLY=( $(compgen -W "-t" -- $cur) )
      else
        _docker_procs
      fi
      return 0
      ;;
    tag)
      # XXX incomplete, second param should be only repos
      if [[ $cur == -* ]]; then
        COMPREPLY=( $(compgen -W "-f" -- $cur) )
      else
        _docker_imgs
      fi
      return 0
      ;;
  esac

  case $cur in
    "-"*)
      COMPREPLY=( $(compgen -W "-help -D -b -p -r" -- ${cur}) )
      return 0
      ;;
    *)
      COMPREPLY=( $(IFS="|" compgen -W "${cmds}" -- ${cur}) )
      return 0
      ;;
  esac
}
complete -F _docker docker
