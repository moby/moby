
HOST:=boot2docker

makescript:
	./parsedocs.sh > make_certs.sh

build: makescript
	docker build -t makecerts .

cert: build
	docker run --rm -it -v $(CURDIR):/data -e HOST=$(HOST) makecerts

certs: cert

run:
	sudo docker -d -D --tlsverify --tlscacert=ca.pem --tlscert=server-cert.pem --tlskey=server-key.pem -H=0.0.0.0:6666 --pidfile=$(pwd)/docker.pid --graph=$(pwd)/graph

client:
	sudo docker --tls --tlscacert=ca.pem --tlscert=cert.pem --tlskey=key.pem   -H=$(HOST):6666 version
	sudo docker --tlsverify --tlscacert=ca.pem --tlscert=cert.pem --tlskey=key.pem   -H=$(HOST):6666 info
	sudo curl https://$(HOST):6666/images/json --cert cert.pem --key key.pem --cacert ca.pem

clean:
	rm ca-key.pem ca.pem ca.srl cert.pem client.csr extfile.cnf key.pem server-cert.pem server-key.pem server.csr
