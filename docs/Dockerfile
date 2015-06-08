FROM docs/base:hugo
MAINTAINER Mary Anthony <mary@docker.com> (@moxiegirl)

# To get the git info for this repo
COPY . /src

COPY . /docs/content/engine/

# Sed to process GitHub Markdown
# 1-2 Remove comment code from metadata block
# 3 Remove .md extension from link text
# 4 Change ](/ to ](/project/ in links
# 5 Change ](word) to ](/project/word)
# 6 Change ](../../ to ](/project/
# 7 Change ](../ to ](/project/word)
# 
# 
RUN find /docs/content/engine -type f -name "*.md" -exec sed -i.old \
    -e '/^<!.*metadata]>/g' \
    -e '/^<!.*end-metadata.*>/g' \
    -e 's/\([(]\)\(.*\)\(\.md\)/\1\2/g' \
    -e 's/\(\]\)\([(]\)\(\/\)/\1\2\/engine\//g' \
    -e 's/\(\][(]\)\([A-z]*[)]\)/\]\(\/engine\/\2/g' \
    -e 's/\(\][(]\)\(\.\.\/\)/\1\/engine\//g' {} \;
