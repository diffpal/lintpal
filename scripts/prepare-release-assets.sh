#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
workspace=.omnidist/default
version=$(cat "$workspace/dist/VERSION")
tag=$(git describe --tags --exact-match HEAD)
if [[ $tag != "v$version" ]]; then
  echo "Expected tag v$version at HEAD, got $tag" >&2
  exit 1
fi

assets="$workspace/release-assets"
mkdir -p "$assets"

copy_target() {
  local os=$1
  local arch=$2
  local source=$3
  local platform=$4
  local name=$5
  cmp "$workspace/dist/$os/$arch/$source" "$workspace/npm/$platform/bin/$source"
  cp "$workspace/dist/$os/$arch/$source" "$assets/$name"
}

copy_target darwin amd64 lintpal '@diffpal/lintpal-darwin-x64' lintpal-darwin-amd64
copy_target darwin arm64 lintpal '@diffpal/lintpal-darwin-arm64' lintpal-darwin-arm64
copy_target linux amd64 lintpal '@diffpal/lintpal-linux-x64' lintpal-linux-amd64
copy_target linux arm64 lintpal '@diffpal/lintpal-linux-arm64' lintpal-linux-arm64
copy_target windows amd64 lintpal.exe '@diffpal/lintpal-win32-x64' lintpal-windows-amd64.exe

cd "$assets"
test "$(find . -maxdepth 1 -type f -name 'lintpal-*' | wc -l)" -eq 5
sha256sum lintpal-darwin-amd64 lintpal-darwin-arm64 \
  lintpal-linux-amd64 lintpal-linux-arm64 lintpal-windows-amd64.exe > checksums.txt
sha256sum -c checksums.txt
../dist/linux/amd64/lintpal version
cat checksums.txt
