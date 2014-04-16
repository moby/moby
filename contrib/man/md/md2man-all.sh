#!/bin/bash
echo "md2man-all: about to mkdir ../man1"
mkdir -p /pandoc/man1
echo "md2man-all: changing to /pandoc/md"
cd /pandoc/md
echo "md2man-all: about to convert files"
for FILE in docker*.md; do echo $FILE; pandoc -s -t man $FILE -o /pandoc/man1/"${FILE%.*}".1; done
echo "md2man-all: List files:"
ls -l /pandoc/man1
