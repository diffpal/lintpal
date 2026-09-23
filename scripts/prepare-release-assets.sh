#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
workspace=.omnidist/default
version=$(cat "$workspace/dist/VERSION")
if [[ $version != 0.1.0 ]]; then
  echo "Expected staged version 0.1.0, got $version" >&2
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

copy_target darwin amd64 jevlint '@diffpal/jevlint-darwin-x64' jevlint-darwin-amd64
copy_target darwin arm64 jevlint '@diffpal/jevlint-darwin-arm64' jevlint-darwin-arm64
copy_target linux amd64 jevlint '@diffpal/jevlint-linux-x64' jevlint-linux-amd64
copy_target linux arm64 jevlint '@diffpal/jevlint-linux-arm64' jevlint-linux-arm64
copy_target windows amd64 jevlint.exe '@diffpal/jevlint-win32-x64' jevlint-windows-amd64.exe

cd "$assets"
test "$(find . -maxdepth 1 -type f -name 'jevlint-*' | wc -l)" -eq 5
sha256sum jevlint-darwin-amd64 jevlint-darwin-arm64 \
  jevlint-linux-amd64 jevlint-linux-arm64 jevlint-windows-amd64.exe > checksums.txt
sha256sum -c checksums.txt
../dist/linux/amd64/jevlint version
cat checksums.txt
