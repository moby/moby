FROM busybox

ADD . /

RUN /generate-new-dockerfile.sh > Dockerfile

BUILD /

# Inner build uses generated Dockerfile