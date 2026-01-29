# Script to create new profile from template

USAGE="Usage: $0 <ARG1>

ARG1: file_name"

if [ $# -eq 0 ]; then
    echo "No arguments provided..."
    echo "$USAGE"
    exit 1
fi

if [ ! -d profiles ]
then
   echo "Directory 'profiles' does not exist. Can't make new file."
   exit 1
fi


if [ -e profiles/profile_$1.go ]
then
   echo "File already exists. Can't make new file."
   exit 1
fi

PROFILE=$1

sed -e "s/PROFILE/${PROFILE}/" profileTemplate > profiles/profile_${PROFILE}.go

echo "Created file profiles/lint_${PROFILE}.go"
