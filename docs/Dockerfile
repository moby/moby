FROM docs/base:oss
ENV PROJECT=engine
# To get the git info for this repo
COPY . /src
RUN rm -rf /docs/content/$PROJECT/
COPY . /docs/content/$PROJECT/
