#!/bin/sh
# Build a distributable (zipped) copy for a specific target
USAGE="USAGE: ./go-build.sh <goos> <goarch> <build-dir> <build-name> <package>"

if [ "${#}" -ne 5 ]
then
  1>&2 echo ${USAGE}
  exit 1
fi
goos=${1}
goarch=${2}
build_dir=${3}
build_name=${4}
package=${5}
build_path="${build_dir}/${build_name}-${goos}-${goarch}"
set -eoux pipefail

GOOS=${goos} GOARCH=${goarch} CGO_ENABLED=0 go build -o ${build_path} ${package}
zip -j ${build_path}.zip ${build_path}