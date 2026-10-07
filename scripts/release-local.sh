#!/bin/sh
set -eu
export GOMAXPROCS=2 GOMEMLIMIT=1500MiB GOFLAGS=-p=2 CGO_ENABLED=0
release_version="${RELEASE_VERSION:-0.1.0}"
case "$release_version" in *[!0-9.]*|'' ) echo "Invalid release version" >&2; exit 1;; esac
mkdir -p dist
for target_os in linux darwin windows; do
 for target_arch in amd64 arm64; do
  extension=; [ "$target_os" != windows ] || extension=.exe
  folder="dist/disclosery_${release_version}_${target_os}_${target_arch}"
  mkdir -p "$folder"
  GOOS="$target_os" GOARCH="$target_arch" go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$release_version" -o "$folder/disclosery$extension" ./cmd/disclosery
  cp README.md LICENSE THIRD_PARTY_NOTICES.md "$folder/"
  if [ "$target_os" = windows ]; then
   go run scripts/package-release.go "$folder"
  else
   tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner -czf "$folder.tar.gz" -C "$folder" .
  fi
 done
done
cd dist
sha256sum disclosery_*.tar.gz disclosery_*.zip > checksums.txt
