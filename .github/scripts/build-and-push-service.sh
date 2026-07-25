#!/usr/bin/env bash
set -Eeuo pipefail

: "${SERVICE:?SERVICE is required}"
: "${ARCH:?ARCH is required}"
: "${TAG:?TAG is required}"
: "${DOCKER_IMAGE:?DOCKER_IMAGE is required}"
: "${BUILDER_NAME:?BUILDER_NAME is required}"

attempt=1
delay=15

print_diagnostics() {
  echo "Collecting Buildx diagnostics for ${SERVICE} (${ARCH})"
  timeout --foreground --signal=TERM --kill-after=10s 30s \
    docker buildx du --builder "${BUILDER_NAME}" || true
  docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}' || true
}

handle_cancel() {
  echo "Build cancelled; stopping Buildx builder ${BUILDER_NAME}"
  timeout --foreground --signal=TERM --kill-after=10s 30s \
    docker buildx stop "${BUILDER_NAME}" || true
  exit 130
}

trap handle_cancel INT TERM

while true; do
  echo "Building ${SERVICE} for ${ARCH}: attempt ${attempt}/4"
  if timeout --foreground --signal=TERM --kill-after=30s 8m docker buildx build \
    --builder "${BUILDER_NAME}" \
    --platform "linux/${ARCH}" \
    --push \
    --progress=plain \
    --provenance=false \
    --no-cache-filter builder \
    --build-arg "APP_PATH=./cmd/${SERVICE}" \
    --build-arg "FRONTEND_REVISION=${GITHUB_SHA}" \
    --label "org.opencontainers.image.created=$(date --utc +%Y-%m-%dT%H:%M:%SZ)" \
    --label "org.opencontainers.image.description=An API unified management platform for provider routing, access control, and usage audit." \
    --label "org.opencontainers.image.licenses=AGPL-3.0" \
    --label "org.opencontainers.image.revision=${GITHUB_SHA}" \
    --label "org.opencontainers.image.source=${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}" \
    --label "org.opencontainers.image.title=codego-api" \
    --label "org.opencontainers.image.url=${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}" \
    --label "org.opencontainers.image.version=${TAG}" \
    --tag "${DOCKER_IMAGE}:${TAG}-${SERVICE}-${ARCH}" \
    --tag "${DOCKER_IMAGE}:latest-${SERVICE}-${ARCH}" \
    .; then
    echo "### Docker Image (${ARCH})" >> "${GITHUB_STEP_SUMMARY}"
    echo "\`${DOCKER_IMAGE}:${TAG}-${SERVICE}-${ARCH}\`" >> "${GITHUB_STEP_SUMMARY}"
    exit 0
  fi

  print_diagnostics
  if [ "${attempt}" -ge 4 ]; then
    echo "::error::Build failed for ${SERVICE} after ${attempt} attempts"
    exit 1
  fi
  echo "Build failed for ${SERVICE}; retrying in ${delay}s"
  sleep "${delay}"
  attempt=$((attempt + 1))
  delay=$((delay * 2))
done
