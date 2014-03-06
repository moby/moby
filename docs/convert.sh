#!/bin/sh

#hugo conversion
#find . -type d -exec mkdir -p ~/src/docker/hugostrap/docker.io/content/docs/'{}' \;
#find . -name '*.rst'  | sed 's/.\/\(.*\).rst/pandoc  --from=rst --to=markdown --reference-links --atx-headers -o ~\/src\/docker\/hugostrap\/docker.io\/content\/docs\/\1.md \1.rst/' | sh



# convert from rst to md for mkdocs.org
# TODO: we're using a sphinx specific rst thing to do between docs links, which we then need to convert to mkdocs specific markup (and pandoc loses it when converting to html / md)
HTML_FILES=$(find _build -name '*.html' | sed 's/_build\/html\/\(.*\)\/index.html/\1/')

for name in ${HTML_FILES} 
do
	echo $name
#	pandoc -f rst -t html -o ${name}.html ${name}.rst
        pandoc -f html -t markdown -o sources/${name}.md _build/html/${name}/index.html

#	rm ${name}.html

	#TODO: remove or fixup the meta-data

	# git it all so we can test
#	git add ${name}.md
done
