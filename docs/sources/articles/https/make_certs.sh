#!/bin/bash

openssl genrsa -aes256 -out ca-key.pem 2048

echo "enter your Docker daemon's hostname as the 'Common Name'= ($HOST)"

#TODO add this as an ENV to docker run?
openssl req -new -x509 -days 365 -key ca-key.pem -sha256 -out ca.pem


# server cert
openssl genrsa -out server-key.pem 2048
openssl req -subj "/CN=$HOST" -new -key server-key.pem -out server.csr
openssl x509 -req -days 365 -in server.csr -CA ca.pem -CAkey ca-key.pem \
  -CAcreateserial -out server-cert.pem

#client cert
openssl genrsa -out key.pem 2048
openssl req -subj '/CN=client' -new -key key.pem -out client.csr

echo extendedKeyUsage = clientAuth > extfile.cnf
openssl x509 -req -days 365 -in client.csr -CA ca.pem -CAkey ca-key.pem \
  -CAcreateserial -out cert.pem -extfile extfile.cnf
