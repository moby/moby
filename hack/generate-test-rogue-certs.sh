#!/bin/bash
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"

OUT_DIR="${SCRIPT_DIR}/../integration-cli/fixtures/https"

# generate CA
echo 01 > "${OUT_DIR}/ca-rogue.srl"
openssl genrsa -out "${OUT_DIR}/ca-rogue-key.pem"

openssl req \
	-new \
	-x509 \
	-days 3652 \
	-subj "/C=US/ST=CA/L=SanFrancisco/O=Evil Inc/OU=changeme/CN=changeme/name=changeme/emailAddress=mail@host.domain" \
	-nameopt compat \
	-text \
	-key "${OUT_DIR}/ca-rogue-key.pem" \
	-out "${OUT_DIR}/ca-rogue.pem"

# Now that we have a CA, create a server key and certificate signing request.
# Make sure that `"Common Name (e.g. server FQDN or YOUR name)"` matches the hostname you will use
# to connect or just use '*' for a certificate valid for any hostname:

openssl genrsa -out "${OUT_DIR}/server-rogue-key.pem"
openssl req -new \
	-subj "/C=US/ST=CA/L=SanFrancisco/O=Evil Inc/OU=changeme/CN=changeme/name=changeme/emailAddress=mail@host.domain" \
	-text \
	-key "${OUT_DIR}/server-rogue-key.pem" \
	-out "${OUT_DIR}/server-rogue.csr"

# Options for server certificate
cat > "${OUT_DIR}/server-rogue-options.cfg" << 'EOF'
basicConstraints=CA:FALSE
subjectKeyIdentifier=hash
authorityKeyIdentifier=keyid,issuer
extendedKeyUsage=serverAuth
subjectAltName=DNS:*,DNS:localhost,IP:127.0.0.1,IP:::1
EOF

# Generate the certificate and sign with our CA
openssl x509 \
	-req \
	-days 3652 \
	-extfile "${OUT_DIR}/server-rogue-options.cfg" \
	-CA "${OUT_DIR}/ca-rogue.pem" \
	-CAkey "${OUT_DIR}/ca-rogue-key.pem" \
	-nameopt compat \
	-text \
	-in "${OUT_DIR}/server-rogue.csr" \
	-out "${OUT_DIR}/server-rogue-cert.pem"

# For client authentication, create a client key and certificate signing request
openssl genrsa -out "${OUT_DIR}/client-rogue-key.pem"
openssl req -new \
	-subj "/C=US/ST=CA/L=SanFrancisco/O=Evil Inc/OU=changeme/CN=changeme/name=changeme/emailAddress=mail@host.domain" \
	-text \
	-key "${OUT_DIR}/client-rogue-key.pem" \
	-out "${OUT_DIR}/client-rogue.csr"

# Options for client certificate
cat > "${OUT_DIR}/client-rogue-options.cfg" << 'EOF'
basicConstraints=CA:FALSE
subjectKeyIdentifier=hash
authorityKeyIdentifier=keyid,issuer
extendedKeyUsage=clientAuth
subjectAltName=DNS:*,DNS:localhost,IP:127.0.0.1,IP:::1
EOF

# Generate the certificate and sign with our CA:
openssl x509 \
	-req \
	-days 3652 \
	-extfile "${OUT_DIR}/client-rogue-options.cfg" \
	-CA "${OUT_DIR}/ca-rogue.pem" \
	-CAkey "${OUT_DIR}/ca-rogue-key.pem" \
	-nameopt compat \
	-text \
	-in "${OUT_DIR}/client-rogue.csr" \
	-out "${OUT_DIR}/client-rogue-cert.pem"

rm "${OUT_DIR}/ca-rogue.srl"
rm "${OUT_DIR}/ca-rogue-key.pem"
rm "${OUT_DIR}"/*.cfg
rm "${OUT_DIR}"/*.csr
