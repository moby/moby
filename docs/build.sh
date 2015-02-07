#!/usr/bin/env bash
set -e

set -o pipefail

usage() {
	exit 1
}


extrafiles=($(find . -name "mkdocs-*.yml"))
extralines=()

for file in "${extrafiles[@]}"
do
	#echo "LOADING $file"
	while read line
	do
		if [[ "$line" != "" ]]
		then
			extralines+=("$line")

			#echo "LINE (${#extralines[@]}):  $line"
		fi
	done < <(cat "$file")
done

#echo "extra count (${#extralines[@]})"
mv mkdocs.yml mkdocs.yml.bak
echo "# Generated mkdocs.yml from ${extrafiles[@]}"
echo "# Generated mkdocs.yml from ${extrafiles[@]}" > mkdocs.yml

while read line
do
	menu=$(echo $line | sed "s/^- \['\([^']*\)', '\([^']*\)'.*/\2/")
	if [[ "$menu" != "**HIDDEN**" ]]
		# or starts with a '#'?
	then
		if [[ "$lastmenu" != "" && "$lastmenu" != "$menu" ]]
		then
			# insert extra elements here
			for extra in "${extralines[@]}"
			do
				#echo "EXTRA $extra"
				extramenu=$(echo $extra | sed "s/^- \['\([^']*\)', '\([^']*\)'.*/\2/")
				if [[ "$extramenu" == "$lastmenu" ]]
				then
					echo "$extra" >> mkdocs.yml
				fi
			done
			#echo "# JUST FINISHED $lastmenu"
		fi
		lastmenu="$menu"
	fi
	echo "$line" >> mkdocs.yml

done < <(cat "mkdocs.yml.bak")
