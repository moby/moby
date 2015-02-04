#!/bin/bash
echo "#!/bin/bash"
echo "cat << 'EOF'"
regex='^(if|elif|else|fi).*$'
regex2='.*\\$'
regex3='(.*)(\$|`)(.*)'
while IFS='' read -r line
do
    if [[ $line =~ $regex ]]; then
        echo "EOF"
        echo "$line" 
	echo "cat << 'EOF'"
    else
        echo "$line"
    fi  
done <$(dirname "$BASH_SOURCE")/../Dockerfile.in
echo "EOF"

