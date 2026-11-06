#!/usr/bin/env bash
set -euo pipefail
root=/mnt/g/programming/go-xml-v2
work=/tmp/go-xml-v2-benchmark
mkdir -p "$work" "$work/assets" "$work/jdk"
if [[ ! -x "$work/bootstrap/bin/go" ]]; then
  curl --fail --location --retry 3 -o "$work/go.tar.gz" https://go.dev/dl/go1.26.0.linux-amd64.tar.gz
  mkdir -p "$work/bootstrap"
  tar -xzf "$work/go.tar.gz" --strip-components=1 -C "$work/bootstrap"
fi
if [[ ! -x "$work/jdk/bin/java" ]]; then
  curl --fail --location --retry 3 -o "$work/jdk.tar.gz" 'https://api.adoptium.net/v3/binary/latest/25/ga/linux/x64/jdk/hotspot/normal/eclipse'
  tar -xzf "$work/jdk.tar.gz" --strip-components=1 -C "$work/jdk"
fi
if [[ ! -d "$work/go-src/.git" ]]; then
  git -c safe.directory="$root" clone --no-hardlinks --no-local "$root" "$work/go-src"
fi
git -C "$work/go-src" checkout --detach bd5149100704087efd1edf06bfb7d6956aa636ac
# Extract the assets as committed (LF), not from the Windows working tree,
# which core.autocrlf may have rewritten to CRLF. run.py checks assets.csv.
ref="$root/pkg/stax-xml-reference"
rm -rf "$work/assets" && mkdir -p "$work/assets"
git -c safe.directory="$ref" -c core.autocrlf=false -c core.eol=lf -C "$ref" \
  archive 07cca8a0f53f83a47de4149f84749e4259dc9360 packages/benchmark/assets |
  tar -x -C "$work/assets" --strip-components=3
cd "$work/go-src/src"
GOROOT_BOOTSTRAP="$work/bootstrap" GOTOOLCHAIN=local CGO_ENABLED=0 ./make.bash
