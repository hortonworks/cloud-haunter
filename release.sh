#! /bin/bash
set -x

NAME=$(basename `git rev-parse --show-toplevel`)
ARCH=$(uname -m)
BRANCH=$(git rev-parse --abbrev-ref HEAD)

rm -rf release
mkdir release

TAR=tar
GREP=grep
# On OS X refer to the gnu utils, they can be installed using brew
if [[ $OSTYPE =~ "darwin" ]]; then
  TAR=gtar
  GREP=ggrep
fi

declare -a Platforms=("Linux" "Darwin")
for platform in ${Platforms[@]}; do
  if [ -d "./build/$platform" ]; then
    echo "Compressing the ${platform} relevant binary ..."
    $TAR -zcf "release/${NAME}_${VERSION}_${platform}_${ARCH}.tgz" -C build/$platform $BINARY
  fi
done

echo "Creating release v${VERSION} from branch $BRANCH ..."

output=$(gh release list | $GREP ${VERSION})
if [ -z "$output" ]; then 
  gh release create "v${VERSION}" "./release/${NAME}_${VERSION}_Linux_${ARCH}.tgz" "./release/${NAME}_${VERSION}_Darwin_${ARCH}.tgz" -t ${VERSION} -n ""
else
  echo "The cli release ${VERSION} already exists on the github."
fi
