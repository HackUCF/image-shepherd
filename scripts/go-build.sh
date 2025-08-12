#!/bin/bash
# Build a distributable (zipped) copy for a specific target
set -euo pipefail

USAGE="USAGE: ./go-build.sh <goos> <goarch> <build-dir> <build-name> <package>"

if [ "${#}" -ne 5 ]; then
  echo "${USAGE}" >&2
  exit 1
fi

goos=${1}
goarch=${2}
build_dir=${3}
build_name=${4}
package=${5}

# Validate inputs
if [ -z "${goos}" ] || [ -z "${goarch}" ] || [ -z "${build_dir}" ] || [ -z "${build_name}" ] || [ -z "${package}" ]; then
  echo "Error: All parameters are required" >&2
  echo "${USAGE}" >&2
  exit 1
fi

# Create build directory if it doesn't exist
mkdir -p "${build_dir}"

# Get version from git or use "dev"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Handle Windows executable extension
binary_name="${build_name}"
if [ "${goos}" = "windows" ] && [[ ! "${build_name}" =~ \.exe$ ]]; then
  binary_name="${build_name}.exe"
fi

echo "Building ${package} for ${goos}/${goarch}..."
echo "Output: ${build_dir}/${binary_name}"
echo "Version: ${VERSION}"

# Build the binary
GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 go build \
  -ldflags="-w -s -X main.version=${VERSION}" \
  -o "${build_dir}/${binary_name}" \
  "${package}"

# Verify the binary was created
if [ ! -f "${build_dir}/${binary_name}" ]; then
  echo "Error: Build failed - binary not found at ${build_dir}/${binary_name}" >&2
  exit 1
fi

# Create zip archive
archive_name="${build_name}-${goos}-${goarch}.zip"
echo "Creating archive: ${build_dir}/${archive_name}"

# Change to build directory to avoid including path in zip
cd "${build_dir}"
zip -q "${archive_name}" "${binary_name}"

# Verify the archive was created
if [ ! -f "${archive_name}" ]; then
  echo "Error: Failed to create archive ${archive_name}" >&2
  exit 1
fi

# Clean up binary (keep only the zip)
rm "${binary_name}"

echo "Successfully created ${build_dir}/${archive_name}"

# Optional: Show archive contents for verification
if command -v unzip >/dev/null 2>&1; then
  echo "Archive contents:"
  unzip -l "${archive_name}"
fi
