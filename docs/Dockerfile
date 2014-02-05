FROM ubuntu:12.04
MAINTAINER Nick Stinemates
#
#    docker build -t docker:docs . && docker run -p 8000:8000 docker:docs
#

# TODO switch to http://packages.ubuntu.com/trusty/python-sphinxcontrib-httpdomain once trusty is released

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -yq make python-pip python-setuptools
# pip installs from docs/requirements.txt, but here to increase cacheability
RUN pip install Sphinx==1.2.1
RUN pip install sphinxcontrib-httpdomain==1.2.0
ADD . /docs
RUN make -C /docs clean docs

WORKDIR /docs/_build/html
CMD ["python", "-m", "SimpleHTTPServer"]
# note, EXPOSE is only last because of https://github.com/dotcloud/docker/issues/3525
EXPOSE 8000
