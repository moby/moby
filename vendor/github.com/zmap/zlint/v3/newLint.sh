# Script to create new lint from template

USAGE="Usage: $0 <ARG1> <ARG2> <ARG3>

ARG1: Path_name
ARG2: File_name/TestName (no 'lint_' prefix)
ARG3: Struct_name"

if [ $# -eq 0 ]; then
    echo "No arguments provided..."
    echo "$USAGE"
    exit 1
fi

if [ $# -eq 1 ]; then
    echo "Not enough arguments provided..."
    echo "$USAGE"
    exit 1
fi

if [ $# -eq 2 ]; then
    echo "Not enough arguments provided..."
    echo "$USAGE"
    exit 1
fi

if [ ! -d lints/$1 ]
then
   echo "Directory 'lints/$1' does not exist. Can't make new file."
   exit 1
fi


if [ -e lints/$1/lint_$2.go ]
then
   echo "File already exists. Can't make new file."
   exit 1
fi

PATHNAME=$1
LINTNAME=$2
# Remove the first two characters from ${LINTNAME} and save the resulting string into FILENAME
FILENAME=${LINTNAME:2}
STRUCTNAME=$3

sed -e "s/PACKAGE/${PATHNAME}/" \
    -e "s/PASCAL_CASE_SUBST/${STRUCTNAME^}/g" \
    -e "s/SUBST/${STRUCTNAME}/g" \
    -e "s/SUBTEST/${LINTNAME}/g" template > lints/${PATHNAME}/lint_${FILENAME}.go

echo "Created file lints/${PATHNAME}/lint_${FILENAME}.go with struct name ${STRUCTNAME}"
