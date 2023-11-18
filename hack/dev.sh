#!/bin/bash

run=1
lastInt=0


function checkEnd() {
    now=$(date +%s)
    if [[ $(($now-$lastInt)) -le 1 ]]; then
        run=0
    fi
    lastInt=$now
}

trap "checkEnd" INT

while [ $run -eq 1 ]; do
    ./hack/make.sh binary || (sleep 5 && continue)
    KEEPBUNDLE=1 ./hack/make.sh install-binary || continue
    dockerd --debug || true
done
