#!/usr/bin/env bash
set -euo pipefail
root=/mnt/g/programming/go-xml-v2
work=/tmp/go-xml-v2-benchmark
source="$root/benchmark-results/windows-amd64/assets-comparison"
export GOROOT="$work/go-src" GOEXPERIMENT=xmlv2 GOAMD64=v1 CGO_ENABLED=0 GO111MODULE=off GOTOOLCHAIN=local GOMAXPROCS=8
export PATH="$work/jdk/bin:$PATH"
mkdir -p "$work/java" "$work/rust"
cp -R "$source/rust/." "$work/rust/"
"$GOROOT/bin/go" version
"$GOROOT/bin/go" test encoding/xml/... -count=1 | tee "$root/benchmark-results/linux-amd64/tests.txt"
"$GOROOT/bin/go" build -o "$work/asset-go" "$source/main.go"
woodstox=/mnt/c/Users/z1z0b/.m2/repository/com/fasterxml/woodstox/woodstox-core/7.2.0/woodstox-core-7.2.0.jar
stax=/mnt/c/Users/z1z0b/.m2/repository/org/codehaus/woodstox/stax2-api/4.3.0/stax2-api-4.3.0.jar
cp "$woodstox" "$work/woodstox.jar"
cp "$stax" "$work/stax2.jar"
javac -cp "$work/woodstox.jar:$work/stax2.jar" -d "$work/java" "$source/AssetBench.java"
CARGO_TARGET_DIR="$work/cargo-target" cargo build --release --locked --manifest-path "$work/rust/Cargo.toml"
python3 "$root/benchmark-results/linux-amd64/run.py"
