#!/bin/bash

# The source_dir is the last segment from repository URL
source_dir=$(echo $SOURCE_URI | grep -o -e "[^/]*$" | sed -e "s/\.git$//")

result=1

if [ -z "${IMAGE_NAME}" ]; then
  echo "[ERROR] The IMAGE_NAME environment variable must be set"
  exit $result
fi

# Clone the STI image repository
git clone $SOURCE_URI
if ! [ $? -eq 0 ]; then
  echo "[ERROR] Unable to clone the STI image repository."
  exit $result
fi


pushd $source_dir >/dev/null
  # Checkout desired ref
  if ! [ -z "$SOURCE_REF" ]; then
    git checkout $SOURCE_REF
  fi

  docker build -t ${IMAGE_NAME}-candidate .
  result=$?
  if ! [ $result -eq 0 ]; then
    echo "[ERROR] Unable to build ${IMAGE_NAME}-candidate image (${result})"
  fi

  # Verify the 'test/run' is present
  if ! [ -x "./test/run" ]; then
    echo "[ERROR] Unable to locate the 'test/run' command for the image"
    exit 1
  fi

  # Execute tests
  IMAGE_NAME=${IMAGE_NAME}-candidate ./test/run
  result=$?
  if [ $result -eq 0 ]; then
    echo "[SUCCESS] ${IMAGE_NAME} image tests executed successfully"
  else
    echo "[FAILURE] ${IMAGE_NAME} image tests failed ($result)"
    exit $result
  fi
popd >/dev/null

# After successfull build, retag the image to 'qa-ready'
#
image_id=$(docker inspect --format="{{ .Id }}" ${IMAGE_NAME}-candidate:latest)
docker tag ${image_id} ${IMAGE_NAME}:qa-ready

# Tag the image with the GIT ref if the SOURCE_REF is set
#
if ! [ -z "${SOURCE_REF}" ]; then
  docker tag ${image_id} ${IMAGE_NAME}:git-$SOURCE_REF
fi

# Remove the candidate image after build
docker rmi ${IMAGE_NAME}-candidate
