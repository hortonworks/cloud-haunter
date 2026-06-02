#! /bin/bash
set -ex

git config --global --add safe.directory '*'

NAME=$(basename `git rev-parse --show-toplevel`)
ARCH=$(uname -m)

rm -rf release
mkdir release

declare -a PLATFORMS=("Linux" "Darwin")
declare -a FILES=()

TAR=tar
GREP=grep
SED=sed
# On OS X refer to the gnu utils, they can be installed using brew
if [[ $OSTYPE =~ "darwin" ]]; then
  TAR=gtar
  GREP=ggrep
  SED=gsed
fi

for PLATFORM in ${PLATFORMS[@]}; do
  if [ -d "./build/$PLATFORM" ]; then
    echo "Compressing the ${PLATFORM} relevant binary ..."
    FILE="${NAME}_${VERSION}_${PLATFORM}_${ARCH}.tgz"
    LATEST_FILE="${NAME}_latest_${PLATFORM}_${ARCH}.tgz"
    FILES+=("$FILE")
    $TAR -zcf "release/${FILE}" -C build/$PLATFORM $BINARY
    cp ./release/$FILE ./release/$LATEST_FILE
  fi
done

if (( ${#FILES[@]} )); then
  echo "Creating release v${VERSION} ..."
else
  echo "No file found to release."
  exit 0
fi

OUTPUT=$(gh release list | $GREP "^${VERSION}" | true)
if [ -z "$OUTPUT" ]; then 
  
  GH_EXTRA_FLAGS=""
  if [[ "$GH_PRE_RELEASE" == "true" ]]; then
    GH_EXTRA_FLAGS="--prerelease"
  fi

  printf -v RELEASABLE_FILES './release/%s ' "${FILES[@]}"
  gh release create "v${VERSION}" $RELEASABLE_FILES -t ${VERSION} -n "" $GH_EXTRA_FLAGS

  if [[ "$GH_PRE_RELEASE" != "true" ]]; then
    FILE_NAME="Makefile"
    SEARCH=${VERSION}
    REPLACE=${VERSION%.*}.$((${VERSION##*.}+1))

    if [[ $SEARCH != "" && $REPLACE != "" ]]; then
      echo "Increasing version from ${SEARCH} to ${REPLACE} in the ${FILE_NAME}"
      SEARCH_TEXT="export VERSION=${SEARCH}"
      REPLACE_TEXT="export VERSION=${REPLACE}"
      $SED -i "s/$SEARCH_TEXT/$REPLACE_TEXT/" $FILE_NAME
    fi
  fi
else
  echo "The cli release v${VERSION} already exists on Github."
fi
