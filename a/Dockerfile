FROM debian:bullseye-slim
RUN apt-get update && apt-get install -y libcap2-bin --no-install-recommends
RUN setcap CAP_NET_BIND_SERVICE=+eip /bin/sleep
