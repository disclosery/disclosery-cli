#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p .devcache/mod .devcache/build
ionice -c3 nice -n19 sudo -n docker run --rm --cpus=2 --memory=2g --memory-swap=2g --user "$(id -u):$(id -g)" -e HOME=/tmp -e RELEASE_VERSION="${RELEASE_VERSION:-0.1.0}" -e LOCAL_RELEASE="${LOCAL_RELEASE:-0}" -e GOCACHE=/src/.devcache/build -e GOMODCACHE=/src/.devcache/mod -e GOTOOLCHAIN=local -e GOMAXPROCS=2 -e GOMEMLIMIT=1500MiB -e GOFLAGS=-p=2 -v "$PWD:/src" -w /src golang:1.26.8-bookworm sh -ec 'go mod tidy; gofmt -w cmd internal; go vet ./...; go test -parallel=2 ./...; mkdir -p dist; CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=${RELEASE_VERSION}" -o dist/disclosery ./cmd/disclosery; if [ "$LOCAL_RELEASE" = 1 ]; then sh scripts/release-local.sh; fi'
