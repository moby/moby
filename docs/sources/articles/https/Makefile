
HOST:=boot2docker

makescript:
	./parsedocs.sh > make_certs.sh

build: makescript
	docker build -t makecerts .

cert: build
	docker run --rm -it -v $(CURDIR):/data -e HOST=$(HOST) makecerts

certs: cert

run:
	docker -d -D --tlsverify --tlscacert=ca.pem --tlscert=server-cert.pem --tlskey=server-key.pem -H=0.0.0.0:6666 --pidfile=$(pwd)/docker.pid --graph=$(pwd)/graph

client:
	docker --tls --tlscacert=ca.pem --tlscert=cert.pem --tlskey=key.pem   -H=$(HOST):6666 version
	docker --tlsverify --tlscacert=ca.pem --tlscert=cert.pem --tlskey=key.pem   -H=$(HOST):6666 info

clean:
	rm ca-key.pem ca.pem ca.srl cert.pem client.csr extfile.cnf key.pem server-cert.pem server-key.pem server.csr
