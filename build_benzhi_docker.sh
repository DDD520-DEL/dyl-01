#!/usr/bin/env bash
# Build the TSDB engine image for linux/amd64 and linux/arm64 from the
# vendored source tree. The build is fully offline (GOPROXY=off inside the
# image), so no network access is required beyond pulling the base images.
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-tsdb-server}"
TAG="${TAG:-latest}"

cd "$(dirname "$0")"

docker build -f benzhi.Dockerfile --platform linux/amd64 \
  -t "${IMAGE_NAME}:${TAG}-amd64" .

docker build -f benzhi.Dockerfile --platform linux/arm64 \
  -t "${IMAGE_NAME}:${TAG}-arm64" .

echo "built ${IMAGE_NAME}:${TAG}-amd64 and ${IMAGE_NAME}:${TAG}-arm64"
