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
for asset in mini-sample.xml sample.xml complex.xml books.xml ptest.xml crlf.xml midsize.xml large.xml; do
  cp "$root/pkg/stax-xml-reference/packages/benchmark/assets/$asset" "$work/assets/$asset"
done
cd "$work/go-src/src"
GOROOT_BOOTSTRAP="$work/bootstrap" GOTOOLCHAIN=local CGO_ENABLED=0 ./make.bash
