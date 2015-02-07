FROM debian

RUN apt-get update && apt-get install -yq openssl

ADD make_certs.sh /


WORKDIR /data
VOLUMES ["/data"]
CMD /make_certs.sh
