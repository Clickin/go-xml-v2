# Builds the Go asset harness at this tree's xmltext (base) and with
# src/encoding/xml/xmltext/decoder.go from -New (new), rebuilds the Rust and
# Java harnesses, and runs run.py. Run from the repository root after the
# toolchain is built and assets-comparison/run.ps1 has extracted the LF assets.
param([Parameter(Mandatory)][string]$New, [int]$Rounds = 9, [string]$Cpus = 'auto', [string]$Out = 'windows-amd64-pinned')
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path "$PSScriptRoot/../..").Path
$src = "$root/benchmark-results/windows-amd64/assets-comparison"
$env:GOROOT = $root; $env:GOEXPERIMENT = 'xmlv2'; $env:GO111MODULE = 'off'
$env:CGO_ENABLED = '0'; $env:GOAMD64 = 'v1'; $env:GOTOOLCHAIN = 'local'; $env:GOMAXPROCS = '8'
$go = "$root/bin/go.exe"
$f = 'src/encoding/xml/xmltext/decoder.go'
if (git -C $root status --porcelain -- $f) { throw "$f has local changes" }
& $go build -o "$root/pkg/ab-go-base.exe" "$src/main.go"; if ($LASTEXITCODE) { throw 'go build base' }
& git -C $root restore --source=$New -- $f; if ($LASTEXITCODE) { throw "restore $f from $New" }
try {
  & $go build -o "$root/pkg/ab-go-new.exe" "$src/main.go"; if ($LASTEXITCODE) { throw 'go build new' }
} finally { & git -C $root restore --source=HEAD -- $f }
$env:CARGO_TARGET_DIR = "$root/pkg/asset-target"
& cargo build --release --locked --manifest-path "$src/rust/Cargo.toml"; if ($LASTEXITCODE) { throw 'cargo build' }
$jars = 'C:/Users/z1z0b/.m2/repository/com/fasterxml/woodstox/woodstox-core/7.2.0/woodstox-core-7.2.0.jar;C:/Users/z1z0b/.m2/repository/org/codehaus/woodstox/stax2-api/4.3.0/stax2-api-4.3.0.jar'
& javac -cp $jars -d "$root/pkg/asset-java" "$src/AssetBench.java"; if ($LASTEXITCODE) { throw 'javac' }
& py -3 "$PSScriptRoot/run.py" --assets "$root/pkg/stax-xml-assets-lf" --go-base "$root/pkg/ab-go-base.exe" --go-new "$root/pkg/ab-go-new.exe" `
  --quick-xml "$root/pkg/asset-target/release/xml-assets-comparison.exe" --java java --java-cp "$root/pkg/asset-java;$jars" `
  --out "$PSScriptRoot/$Out" --rounds $Rounds --cpus $Cpus
if ($LASTEXITCODE) { throw 'run.py' }
