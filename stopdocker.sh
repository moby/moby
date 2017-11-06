sudo kill $(ps aux | grep 'docker' | sed 's/\s\+/ /g' | cut -d' ' -f2)
