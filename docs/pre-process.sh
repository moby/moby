#!/bin/bash -ex

# Populate an array with just docker dirs and one with content dirs
content_dir=(`ls -d /docs/content/*`)

# Loop content not of docker/
#
# Sed to process GitHub Markdown
# 1-2 Remove comment code from metadata block
# 3 Remove .md extension from link text
# 4 Change ](/ to ](/project/ in links
# 5 Change ](word) to ](/project/word)
# 6 Change ](../../ to ](/project/
# 7 Change ](../ to ](/project/word)
# 
for i in "${content_dir[@]}"
do
   :
   case $i in
      "/docs/content/docker-trusted-registry")
      ;;
      "/docs/content/docker-hub")
      ;;
      "/docs/content/windows")
      ;;
      "/docs/content/mac")
      ;;
      "/docs/content/linux")
      ;;
      "/docs/content/registry")
      y=${i##*/}
      find $i -type f -name "*.md" -not -name "*.compare.md" -exec sed -i.old \
        -e '/^<!\(--\)\{0,1\}\[\(end-\)\{0,1\}metadata\]\(--\)\{0,1\}>/g' \
        -e 's/\(\][(]\)\(\.*\/\)*/\1/g' \
        -e 's/\(\][(]\)\([A-Za-z0-9_/-]\{1,\}\)\(\.md\)\{0,1\}\(#\{0,1\}\(#[A-Za-z0-9_-]*\)\{0,1\}\)[)]/\1\/'$y'\/\2\4)/g' \
        {} \;
      ;;
      "/docs/content/compose")
         y=${i##*/}
        find $i -type f -name "*.md" -exec sed -i.old \
        -e '/^<!.*metadata]>/g' \
        -e '/^<!.*end-metadata.*>/g' \
        -e 's/\(\]\)\([(]\)\(\/\)/\1\2\/'$y'\//g' \
        -e 's/\(\][(]\)\([A-z].*\)\(\.md\)/\1\/'$y'\/\2/g' \
        -e 's/\([(]\)\(.*\)\(\.md\)/\1\2/g'  \
        -e 's/\(\][(]\)\(\.\/\)/\1\/'$y'\//g' \
        -e 's/\(\][(]\)\(\.\.\/\.\.\/\)/\1\/'$y'\//g' \
        -e 's/\(\][(]\)\(\.\.\/\)/\1\/'$y'\//g' {} \;      
      ;;
      "/docs/content/swarm")
         y=${i##*/}
         find $i -type f -name "*.md" -exec sed -i.old \
        -e '/^<!.*metadata]>/g' \
        -e '/^<!.*end-metadata.*>/g' \
        -e 's/\(\]\)\([(]\)\(\/\)/\1\2\/'$y'\//g' \
        -e 's/\(\][(]\)\([A-z].*\)\(\.md\)/\1\/'$y'\/\2/g' \
        -e 's/\([(]\)\(.*\)\(\.md\)/\1\2/g'  \
        -e 's/\(\][(]\)\(\.\/\)/\1\/'$y'\//g' \
        -e 's/\(\][(]\)\(\.\.\/\.\.\/\)/\1\/'$y'\//g' \
        -e 's/\(\][(]\)\(\.\.\/\)/\1\/'$y'\//g' {} \;     
      ;;
      "/docs/content/machine")
         y=${i##*/}
        find $i -type f -name "*.md" -exec sed -i.old \
        -e '/^<!.*metadata]>/g' \
        -e '/^<!.*end-metadata.*>/g' \
        -e 's/\(\]\)\([(]\)\(\/\)/\1\2\/'$y'\//g' \
        -e 's/\(\][(]\)\([A-z].*\)\(\.md\)/\1\/'$y'\/\2/g' \
        -e 's/\([(]\)\(.*\)\(\.md\)/\1\2/g'  \
        -e 's/\(\][(]\)\(\.\/\)/\1\/'$y'\//g' \
        -e 's/\(\][(]\)\(\.\.\/\.\.\/\)/\1\/'$y'\//g' \
        -e 's/\(\][(]\)\(\.\.\/\)/\1\/'$y'\//g' {} \;         
      ;;
      "/docs/content/kitematic")
         y=${i##*/}
        find $i -type f -name "*.md" -exec sed -i.old \
        -e '/^<!.*metadata]>/g' \
        -e '/^<!.*end-metadata.*>/g' \
        -e 's/\(\]\)\([(]\)\(\/\)/\1\2\/'$y'\//g' \
        -e 's/\(\][(]\)\([A-z].*\)\(\.md\)/\1\/'$y'\/\2/g' \
        -e 's/\([(]\)\(.*\)\(\.md\)/\1\2/g'  \
        -e 's/\(\][(]\)\(\.\/\)/\1\/'$y'\//g' \
        -e 's/\(\][(]\)\(\.\.\/\.\.\/\)/\1\/'$y'\//g' \
        -e 's/\(\][(]\)\(\.\.\/\)/\1\/'$y'\//g' {} \;         
      ;;
      "/docs/content/opensource")
         y=${i##*/}
        find $i -type f -name "*.md" -exec sed -i.old \
        -e '/^<!.*metadata]>/g' \
        -e '/^<!.*end-metadata.*>/g' \
        -e 's/\(\]\)\([(]\)\(\/\)/\1\2\/'$y'\//g' \
        -e 's/\(\][(]\)\([A-z].*\)\(\.md\)/\1\/'$y'\/\2/g' \
        -e 's/\([(]\)\(.*\)\(\.md\)/\1\2/g'  \
        -e 's/\(\][(]\)\(\.\/\)/\1\/'$y'\//g' \
        -e 's/\(\][(]\)\(\.\.\/\.\.\/\)/\1\/'$y'\//g' \
        -e 's/\(\][(]\)\(\.\.\/\)/\1\/'$y'\//g' {} \;         
      ;;
      *)
         y=${i##*/}
        find $i -type f -name "*.md" -exec sed -i.old \
        -e '/^<!.*metadata]>/g' \
        -e '/^<!.*end-metadata.*>/g' {} \;        
      ;;
      esac
done


