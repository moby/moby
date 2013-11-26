from ubuntu:12.04
maintainer Nick Stinemates
#
#    docker build -t docker:docs . && docker run -p 8000:8000 docker:docs
#

run apt-get update
run apt-get install -y python-setuptools make
run easy_install pip
#from docs/requirements.txt, but here to increase cacheability
run pip install Sphinx==1.1.3
run pip install sphinxcontrib-httpdomain==1.1.9
add . /docs
run cd /docs; make docs

expose 8000

workdir /docs/_build/html

entrypoint ["python", "-m", "SimpleHTTPServer"]
