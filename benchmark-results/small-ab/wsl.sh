#!/usr/bin/env bash
# WSL counterpart of windows.ps1. Run in one WSL session:
#   bash /mnt/g/programming/go-xml-v2/benchmark-results/small-ab/wsl.sh <new-commit>
# It prepares /tmp/go-xml-v2-benchmark with ../linux-amd64/setup.sh (Go at
# bd51491007, JDK, LF assets), builds the Go harness at that tree (base) and
# with decoder.go from <new-commit> (new), rebuilds Rust and Java, and runs run.py.
set -euo pipefail
new=${1:?usage: wsl.sh <commit with the xmltext change>}
rounds=${2:-3}
root=/mnt/g/programming/go-xml-v2
work=/tmp/go-xml-v2-benchmark
src="$root/benchmark-results/windows-amd64/assets-comparison"
bash "$root/benchmark-results/linux-amd64/setup.sh"
export GOROOT="$work/go-src" GOEXPERIMENT=xmlv2 GOAMD64=v1 CGO_ENABLED=0 GO111MODULE=off GOTOOLCHAIN=local GOMAXPROCS=8
export PATH="$work/jdk/bin:$PATH"
f=src/encoding/xml/xmltext/decoder.go
trap 'git -C "$GOROOT" checkout -- "$f"' EXIT
git -C "$GOROOT" checkout -- "$f"
"$GOROOT/bin/go" build -o "$work/ab-go-base" "$src/main.go"
git -c safe.directory="$root" -C "$root" show "$new:$f" > "$GOROOT/$f"
"$GOROOT/bin/go" build -o "$work/ab-go-new" "$src/main.go"
git -C "$GOROOT" checkout -- "$f"
rm -rf "$work/rust" && mkdir -p "$work/rust" "$work/java"
cp -R "$src/rust/." "$work/rust/"
CARGO_TARGET_DIR="$work/cargo-target" cargo build --release --locked --manifest-path "$work/rust/Cargo.toml"
cp /mnt/c/Users/z1z0b/.m2/repository/com/fasterxml/woodstox/woodstox-core/7.2.0/woodstox-core-7.2.0.jar "$work/woodstox.jar"
cp /mnt/c/Users/z1z0b/.m2/repository/org/codehaus/woodstox/stax2-api/4.3.0/stax2-api-4.3.0.jar "$work/stax2.jar"
javac -cp "$work/woodstox.jar:$work/stax2.jar" -d "$work/java" "$src/AssetBench.java"
python3 "$root/benchmark-results/small-ab/run.py" --assets "$work/assets" --go-base "$work/ab-go-base" \
  --go-new "$work/ab-go-new" --quick-xml "$work/cargo-target/release/xml-assets-comparison" \
  --java "$work/jdk/bin/java" --java-cp "$work/java:$work/woodstox.jar:$work/stax2.jar" \
  --out "$root/benchmark-results/small-ab/linux-amd64" --rounds "$rounds"
