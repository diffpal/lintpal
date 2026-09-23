#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
if [[ $(uname -s) != Linux || $(uname -m) != x86_64 ]]; then
  echo 'Local npm install smoke currently requires Linux amd64' >&2
  exit 1
fi
workspace=.omnidist/default
version=$(cat "$workspace/dist/VERSION")
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
export npm_config_cache="$tmpdir/npm-cache"

node - "$workspace/npm" "$version" <<'NODE'
const fs = require('node:fs');
const path = require('node:path');
const [root, version] = process.argv.slice(2);
const read = name => JSON.parse(fs.readFileSync(path.join(root, name, 'package.json')));
const primary = read('lintpal');
const alias = read('@diffpal/lintpal');
if (primary.name !== 'lintpal' || alias.name !== '@diffpal/lintpal' ||
    primary.version !== version || alias.version !== version ||
    JSON.stringify(primary.optionalDependencies) !== JSON.stringify(alias.optionalDependencies) ||
    primary.bin.lintpal !== alias.bin.lintpal) {
  throw new Error('npm meta-package identity or dependency mismatch');
}
const targets = Object.keys(primary.optionalDependencies);
if (targets.length !== 5 || targets.some(name => !name.startsWith('@diffpal/lintpal-') ||
    primary.optionalDependencies[name] !== version || read(name).version !== version)) {
  throw new Error('staged platform package set or version mismatch');
}
console.log(`Shared npm platform packages: ${targets.join(', ')}`);
NODE

find "$workspace/dist" -type f -name 'lintpal*' -print0 |
  sort -z | xargs -0 sha256sum > "$tmpdir/checksums.txt"
test "$(wc -l < "$tmpdir/checksums.txt")" -eq 5
cat "$tmpdir/checksums.txt"
cmp "$workspace/dist/linux/amd64/lintpal" \
  "$workspace/npm/@diffpal/lintpal-linux-x64/bin/lintpal"

pack() {
  local name=$1
  local package_dir=$2
  local filename
  filename=$(npm pack "$package_dir" --offline --ignore-scripts \
    --pack-destination "$tmpdir" --json | node -e \
    'let x="";process.stdin.on("data",b=>x+=b).on("end",()=>process.stdout.write(JSON.parse(x)[0].filename))')
  printf '%s/%s' "$tmpdir" "$filename"
}

platform=$(pack platform "$workspace/npm/@diffpal/lintpal-linux-x64")
primary=$(pack primary "$workspace/npm/lintpal")
alias=$(pack alias "$workspace/npm/@diffpal/lintpal")

for identity in primary alias; do
  install_dir="$tmpdir/$identity"
  mkdir -p "$install_dir"
  if [[ $identity == primary ]]; then
    meta=$primary
  else
    meta=$alias
  fi
  npm install --prefix "$install_dir" --offline --ignore-scripts \
    --no-audit --no-fund --no-save "$platform" "$meta"
  actual=$("$install_dir/node_modules/.bin/lintpal" version)
  test "$actual" = "lintpal $version"
  "$install_dir/node_modules/.bin/lintpal" --help > "$tmpdir/$identity-help.txt"
  echo "$identity local install: $actual"
done
cmp "$tmpdir/primary-help.txt" "$tmpdir/alias-help.txt"
