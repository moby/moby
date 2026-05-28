#!/bin/bash -e

# Not supported by flock:
# - plan9/*
# - js/wasm
# - wasp1/wasm

for row in $(go tool dist list -json | jq -r '.[] | @base64'); do
  _jq() {
    echo ${row} | base64 --decode | jq -r ${1}
  }

  GOOS=$(_jq '.GOOS')
  GOARCH=$(_jq '.GOARCH')

  echo "$GOOS/$GOARCH"
  GOOS=$GOOS GOARCH=$GOARCH go build
done
