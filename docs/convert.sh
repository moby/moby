#!/bin/sh

cd /

#run the sphinx build first
make -C /docs clean docs

cd /docs

#find sources -name '*.md*' -exec rm '{}' \;

# convert from rst to md for mkdocs.org
# TODO: we're using a sphinx specific rst thing to do between docs links, which we then need to convert to mkdocs specific markup (and pandoc loses it when converting to html / md)
HTML_FILES=$(find _build -name '*.html' | sed 's/_build\/html\/\(.*\)\/index.html/\1/')

for name in ${HTML_FILES} 
do
	echo $name
	# lets not use gratuitious unicode quotes that cause terrible copy and paste issues
	sed -i 's/&#8220;/"/g' _build/html/${name}/index.html
	sed -i 's/&#8221;/"/g' _build/html/${name}/index.html
        pandoc -f html -t markdown --atx-headers -o sources/${name}.md1 _build/html/${name}/index.html

	#add the meta-data from the rst
	egrep ':(title|description|keywords):' sources/${name}.rst | sed 's/^:/page_/' > sources/${name}.md
	echo >> sources/${name}.md
	#cat sources/${name}.md1 >> sources/${name}.md
	# remove the paragraph links from the source
	cat sources/${name}.md1 | sed 's/\[..\](#.*)//' >> sources/${name}.md

	rm sources/${name}.md1

	sed -i 's/{.docutils .literal}//g' sources/${name}.md
	sed -i 's/{.docutils$//g' sources/${name}.md
	sed -i 's/^.literal} //g' sources/${name}.md
	sed -i 's/`{.descname}`//g' sources/${name}.md
	sed -i 's/{.descname}//g' sources/${name}.md
	sed -i 's/{.xref}//g' sources/${name}.md
	sed -i 's/{.xref .doc .docutils .literal}//g' sources/${name}.md
	sed -i 's/{.xref .http .http-post .docutils$//g' sources/${name}.md
	sed -i 's/^    .literal}//g' sources/${name}.md

	sed -i 's/\\\$container\\_id/\$container_id/' sources/examples/hello_world.md
	sed -i 's/\\\$TESTFLAGS/\$TESTFLAGS/' sources/contributing/devenvironment.md
	sed -i 's/\\\$MYVAR1/\$MYVAR1/g' sources/reference/commandline/cli.md

	# git it all so we can test
#	git add ${name}.md
done

#annoyingly, there are lots of failures
patch --fuzz 50 -t -p2 < pr4923.patch || true
patch --fuzz 50 -t -p2 < asciinema.patch || true
