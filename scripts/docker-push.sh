#!/bin/bash
# Push a built docker image to a docker registry
USAGE="USAGE: ./docker-push.sh <image-name>"

if [ "${#}" -ne 1 ]
then
  1>&2 echo ${USAGE}
fi
image_name=${1}
set -eoux pipefail

# Find the SHA for HEAD
HEAD_SHA=$(git show-ref --head | grep HEAD | cut -d ' ' -f 1)

# Check if we're on a tag
TAG_REF=$(git show-ref | grep ${HEAD_SHA} | { grep ' refs/tags/' || test ${?} = 1; } | cut -d ' ' -f 2)
if [ -z "${TAG_REF}" ]
then
  # This isn't a tagged build (i.e. for a release), so use `latest`
  VERSION="latest"
else
  # Strip `ref/tags/` prefix from ref and `v` prefix from tag
  VERSION=$(echo ${TAG_REF} | cut -d'/' -f 3 | sed -e 's/^v//')
fi

docker tag ${image_name} ${image_name}:${VERSION}
docker push ${image_name}:${VERSION}