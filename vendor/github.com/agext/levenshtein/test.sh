set -ev

if [[ "$1" == "goveralls" ]]; then
	echo "Testing with goveralls..."
	go get github.com/mattn/goveralls
	$HOME/gopath/bin/goveralls -service=travis-ci
else
	echo "Testing with go test..."
	go test -v ./...
fi
